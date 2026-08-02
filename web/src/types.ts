export type Snapshot={time:string;cpu_percent:number;load1:number;memory_total:number;memory_used:number;swap_total:number;swap_used:number;disk_total:number;disk_used:number;net_rx:number;net_tx:number;uptime:number}
export type Service={Name:string;Version:string;Path:string;Status:string;ConfigPath:string;Installed:boolean}
export type Container={id:string;names:string[];image:string;state:string;status:string}
export type Website={id:string;server:string;name:string;domains:string[];listen:string[];proxy_target:string;tls:boolean;enabled:boolean;source_path:string;raw:string}
export type Task={id:string;kind:string;status:string;summary:string;log:string;created_at:string;updated_at:string}
export type Audit={id:number;Actor:string;Action:string;Resource:string;Detail:string;RemoteIP:string;CreatedAt:string}
export type Me={username:string;must_change:boolean;csrf_token:string;totp_enabled:boolean}
export type AlertRule={ID:number;Name:string;Metric:string;Operator:string;Threshold:number;DurationSeconds:number;SilenceSeconds:number;RepeatSeconds:number;Enabled:boolean}
