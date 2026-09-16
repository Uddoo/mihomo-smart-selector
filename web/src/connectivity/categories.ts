// Product categories describe the service, not the probe URL or network route.
export const categories = [
  {id: 'ai', name: 'AI', description: '对话、模型与创作'},
  {id: 'social', name: '社交', description: '消息、社区与社交网络'},
  {id: 'media', name: '影音', description: '视频、直播与音乐'},
  {id: 'development', name: '开发与云', description: '代码、软件包与云基础设施'},
  {id: 'information', name: '搜索资讯', description: '搜索、知识与新闻'},
  {id: 'shopping', name: '购物', description: '电商与在线购物'},
  {id: 'gaming', name: '游戏', description: '游戏平台与主机服务'},
  {id: 'tools', name: '工具', description: '设备生态与协作工具'},
] as const

export type CategoryID = typeof categories[number]['id']
export type CategoryFilter = CategoryID | 'all'
export type ServiceView = 'all' | 'mine'
