import { it, expect } from 'vitest'
import { applyProvider, storageReady, BLANK_STORAGE } from '@/utils/storageTypes'

it('换服务商：地域、Endpoint、路径风格跟着变', () => {
  const st = { ...BLANK_STORAGE, region: 'x', endpoint: 'https://old' }
  applyProvider(st, 'tencent')
  expect(st).toMatchObject({ provider: 'tencent', region: 'ap-shanghai', endpoint: '', pathStyle: false })
  applyProvider(st, 'minio')
  expect(st).toMatchObject({ provider: 'minio', region: '', pathStyle: true })
})

it('storageReady：有地域规则的填地域就行，R2/MinIO 必须填 Endpoint', () => {
  const base = { ...BLANK_STORAGE, bucket: 'b', accessKeyId: 'ak' }
  expect(storageReady({ ...base, provider: 'aliyun', region: 'cn-shanghai' })).toBe(true)
  expect(storageReady({ ...base, provider: 'aliyun' })).toBe(false)
  expect(storageReady({ ...base, provider: 'r2', region: 'auto' })).toBe(false)
  expect(storageReady({ ...base, provider: 'r2', endpoint: 'https://a.r2.cloudflarestorage.com' })).toBe(true)
  expect(storageReady({ ...base, provider: 'aliyun', region: 'cn-shanghai', bucket: '' })).toBe(false)
})
