// 云端存储：统一走 S3 兼容协议，界面上按"服务商"区分，只影响要填哪些参数、默认值是什么。
// 名称和说明文字在 i18n 的 storage.providers.<value> 下。
// 以后加服务商：在这里加一项 + 两份语言文件加文字（后端 internal/storage 的 Provider 常量和 Endpoint 规则也要对上）。
// 后端的 local（本地文件夹）只给测试用，不在界面展示。
//
//	region        有值：要填地域（这是默认值），Endpoint 可留空，后端按 endpoint(region) 自动生成
//	endpointHint  Endpoint 必填时输入框里的灰色示例
//	pathStyle     默认勾选"路径风格 URL"
export const PROVIDERS = [
  { value: 'aliyun', region: 'cn-shanghai', endpoint: (r) => `https://s3.oss-${r}.aliyuncs.com` },
  { value: 'tencent', region: 'ap-shanghai', endpoint: (r) => `https://cos.${r}.myqcloud.com` },
  { value: 'r2', endpointHint: 'https://<ACCOUNT_ID>.r2.cloudflarestorage.com' },
  { value: 'aws', region: 'us-east-1', endpoint: (r) => `https://s3.${r}.amazonaws.com` },
  { value: 'minio', endpointHint: 'http://192.168.1.10:9000', pathStyle: true },
  { value: 'other', endpointHint: 'https://s3.example.com' },
]

export const providerOf = (value) => PROVIDERS.find((p) => p.value === value) || PROVIDERS[PROVIDERS.length - 1]

// 这些服务商要手填 Endpoint（没有固定的地址规则）
export const needsEndpoint = (p) => !p.endpoint

// storageReady 必填项都填了没有（向导"下一步"、设置页"测试连接"前用）
export function storageReady(st) {
  const p = providerOf(st.provider)
  if (!st.bucket || !st.accessKeyId) return false
  return needsEndpoint(p) ? !!(st.endpoint || '').trim() : !!((st.region || '').trim() || (st.endpoint || '').trim())
}

// 全部字段的空白值。设置页用它垫底再盖上已保存的配置：不能用 DEFAULT_STORAGE 垫底，
// 否则没填地域的配置（如 R2）会被悄悄塞进默认地域，一保存签名就错了。
export const BLANK_STORAGE = {
  type: 's3', provider: 'other', region: '', endpoint: '', bucket: '',
  accessKeyId: '', accessKeySecret: '', pathStyle: false, prefix: 'v1/', path: '',
}

// 向导的初始值：默认阿里云 OSS
export const DEFAULT_STORAGE = { ...BLANK_STORAGE, provider: 'aliyun', region: 'cn-shanghai' }

// applyProvider 换服务商：地域、Endpoint 跟着服务商走，换成新服务商的默认值
export function applyProvider(st, value) {
  const p = providerOf(value)
  Object.assign(st, { provider: value, region: p.region || '', endpoint: '', pathStyle: !!p.pathStyle })
}

// README 里各服务商的开通教程（锚点在 README 里用 <a id="cloud-storage-setup"> 固定住，不随标题文字变）
export const storageGuideURL = (locale) =>
  'https://github.com/ALFO111111/agent-sync/blob/HEAD/' + // HEAD = 默认分支，不管叫 main 还是 master

  (locale === 'zh-CN' ? 'README.md' : 'README_EN.md') + '#cloud-storage-setup'
