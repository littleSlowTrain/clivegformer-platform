export const formatBytes=(value:number)=>{if(!value)return'0 B';const units=['B','KB','MB','GB','TB'];const index=Math.min(Math.floor(Math.log(value)/Math.log(1024)),units.length-1);return`${(value/1024**index).toFixed(index?1:0)} ${units[index]}`}
export const formatDate=(unix:number)=>unix?new Intl.DateTimeFormat('zh-CN',{dateStyle:'medium',timeStyle:'short'}).format(new Date(unix*1000)):'—'

