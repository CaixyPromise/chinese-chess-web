export const initialUsers = [
 {id:'user-1042',name:'听雨观棋',email:'chess.friend@example.test',status:'正常',sessions:2,time:'今天 14:32'},
 {id:'user-1038',name:'山间一局',email:'mountain@example.test',status:'已封禁',sessions:0,time:'昨天 20:15'},
 {id:'user-1031',name:'南城棋友',email:'south@example.test',status:'正常',sessions:1,time:'昨天 18:06'},
];
export const initialTasks = [
 {id:'analysis-2081',name:'听雨观棋',status:'运行中',progress:'18 / 42',time:'今天 14:30'},
 {id:'analysis-2080',name:'南城棋友',status:'排队中',progress:'0 / 36',time:'今天 14:29'},
 {id:'analysis-2074',name:'听雨观棋',status:'失败',progress:'12 / 58',time:'昨天 20:10'},
];
export const initialAudit = [
 {id:'audit-301',name:'管理员',status:'已完成',action:'暂停任务',target:'analysis-2070',reason:'排查重复任务',time:'今天 13:20'},
 {id:'audit-300',name:'管理员',status:'已完成',action:'封禁账号',target:'user-1038',reason:'试用期间异常请求',time:'昨天 20:18'},
];
