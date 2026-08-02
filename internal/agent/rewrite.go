package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/matthewlu070111/anpanel/internal/domain"
)

// Pseudo-static / rewrite templates (BT-style).
func rewriteTemplates() []domain.RewriteRule {
	return []domain.RewriteRule{
		{
			ID: "none", Name: "无", Description: "不使用伪静态",
			Nginx:  "  location / {\n    try_files $uri $uri/ =404;\n  }\n",
			Apache: "  # no rewrite\n",
		},
		{
			ID: "spa", Name: "前端 SPA", Description: "Vue / React / Angular 单页应用",
			Nginx: `  location / {
    try_files $uri $uri/ /index.html;
  }
`,
			Apache: `  FallbackResource /index.html
`,
		},
		{
			ID: "wordpress", Name: "WordPress", Description: "WordPress 固定链接",
			Nginx: `  location / {
    try_files $uri $uri/ /index.php?$args;
  }
`,
			Apache: `  <IfModule mod_rewrite.c>
    RewriteEngine On
    RewriteRule .* - [E=HTTP_AUTHORIZATION:%{HTTP:Authorization}]
    RewriteBase /
    RewriteRule ^index\.php$ - [L]
    RewriteCond %{REQUEST_FILENAME} !-f
    RewriteCond %{REQUEST_FILENAME} !-d
    RewriteRule . /index.php [L]
  </IfModule>
`,
		},
		{
			ID: "thinkphp", Name: "ThinkPHP", Description: "ThinkPHP 5/6 伪静态",
			Nginx: `  location / {
    if (!-e $request_filename) {
      rewrite ^(.*)$ /index.php?s=$1 last;
      break;
    }
  }
`,
			Apache: `  <IfModule mod_rewrite.c>
    RewriteEngine on
    RewriteCond %{REQUEST_FILENAME} !-d
    RewriteCond %{REQUEST_FILENAME} !-f
    RewriteRule ^(.*)$ index.php?s=$1 [QSA,PT,L]
  </IfModule>
`,
		},
		{
			ID: "laravel", Name: "Laravel", Description: "Laravel 路由伪静态",
			Nginx: `  location / {
    try_files $uri $uri/ /index.php?$query_string;
  }
`,
			Apache: `  <IfModule mod_rewrite.c>
    RewriteEngine On
    RewriteCond %{REQUEST_FILENAME} !-d
    RewriteCond %{REQUEST_FILENAME} !-f
    RewriteRule ^ index.php [L]
  </IfModule>
`,
		},
		{
			ID: "yii", Name: "Yii2", Description: "Yii2 Advanced / Basic",
			Nginx: `  location / {
    try_files $uri $uri/ /index.php$is_args$args;
  }
`,
			Apache: `  <IfModule mod_rewrite.c>
    RewriteEngine on
    RewriteCond %{REQUEST_FILENAME} !-f
    RewriteCond %{REQUEST_FILENAME} !-d
    RewriteRule . index.php
  </IfModule>
`,
		},
	}
}

func rewriteByID(id string) (domain.RewriteRule, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		id = "none"
	}
	for _, r := range rewriteTemplates() {
		if r.ID == id {
			return r, nil
		}
	}
	return domain.RewriteRule{}, fmt.Errorf("unknown rewrite rule %q", id)
}

func nginxLocationBlock(rewriteID string) string {
	r, err := rewriteByID(rewriteID)
	if err != nil {
		r, _ = rewriteByID("none")
	}
	return r.Nginx
}

func apacheRewriteBlock(rewriteID string) string {
	r, err := rewriteByID(rewriteID)
	if err != nil {
		r, _ = rewriteByID("none")
	}
	return r.Apache
}

// setSiteRewrite updates AnPanel-managed site config rewrite section.
func setSiteRewrite(ctx context.Context, domain, rewriteID, server string) (ActionResult, error) {
	domain = normalizedDomain(domain)
	if !domainName.MatchString(domain) {
		return ActionResult{}, errors.New("invalid domain name")
	}
	if server == "" {
		var err error
		server, err = preferredWebServer()
		if err != nil {
			return ActionResult{}, err
		}
	}
	if _, err := rewriteByID(rewriteID); err != nil {
		return ActionResult{}, err
	}
	path, err := siteConfigPath(server, domain)
	if err != nil {
		return ActionResult{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ActionResult{}, err
	}
	content := string(raw)
	// Replace between markers if present; else inject before last closing of server/vhost is hard — rewrite whole static root block by regenerating when markers exist.
	const begin = "# BEGIN AnPanel rewrite"
	const end = "# END AnPanel rewrite"
	block := begin + "\n" + strings.TrimRight(map[string]string{"nginx": nginxLocationBlock(rewriteID), "apache": apacheRewriteBlock(rewriteID)}[server], "\n") + "\n" + end + "\n"
	if strings.Contains(content, begin) && strings.Contains(content, end) {
		i := strings.Index(content, begin)
		j := strings.Index(content, end)
		if i >= 0 && j > i {
			j += len(end)
			// include trailing newline
			for j < len(content) && (content[j] == '\n' || content[j] == '\r') {
				j++
			}
			content = content[:i] + block + content[j:]
		}
	} else if server == "nginx" {
		// replace simple location / { ... } once
		content = replaceNginxLocation(content, block)
	} else {
		// inject before </Directory> or </VirtualHost>
		if strings.Contains(content, "</Directory>") {
			content = strings.Replace(content, "</Directory>", block+"</Directory>", 1)
		} else {
			content = strings.Replace(content, "</VirtualHost>", block+"</VirtualHost>", 1)
		}
	}
	return applyWebConfig(ctx, path, content)
}

func replaceNginxLocation(content, newBlock string) string {
	// crude: find first "location / {" and matching brace
	idx := strings.Index(content, "location /")
	if idx < 0 {
		// insert before last }
		li := strings.LastIndex(content, "}")
		if li < 0 {
			return content + "\n" + newBlock
		}
		return content[:li] + newBlock + content[li:]
	}
	brace := strings.Index(content[idx:], "{")
	if brace < 0 {
		return content
	}
	start := idx
	i := idx + brace
	depth := 0
	end := -1
	for ; i < len(content); i++ {
		if content[i] == '{' {
			depth++
		} else if content[i] == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}
	if end < 0 {
		return content
	}
	return content[:start] + strings.TrimRight(newBlock, "\n") + "\n" + content[end:]
}
