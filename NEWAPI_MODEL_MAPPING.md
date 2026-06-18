# NewAPI 模型映射配置

下面这份 JSON 已移除 `seedance-2-0-720` 后面的零宽空格隐藏字符，可直接复制到 NewAPI 模型映射配置中。

```json
{
  "veofast": "doubao-seedance-2-0-260128-2",
  "veo": "doubao-seedance-2-0-260128-3",
  "veofastyang": "doubao-seedance-2-0-260128-2",
  "veoyang": "doubao-seedance-2-0-260128-3",
  "veofastcn-480p": "doubao-seedance-2-0-fast-260128-480p",
  "veocn-480p": "doubao-seedance-2-0-260128-480p",
  "veofastcn-720p": "doubao-seedance-2-0-fast-260128-720p",
  "veocn-720p": "doubao-seedance-2-0-260128-720p",
  "veocnay-480p": "doubao-seedance-2-0-260128-480p",
  "veocnay-720p": "doubao-seedance-2-0-260128-720p",
  "veocn2-720p": "seedance-2-0-720",
  "veofastcn2-720p": "seedance-2-0-fast-720",
  "veocn2ay-720p": "seedance-2-0-720",
  "veofastcn2ay-720p": "seedance-2-0-fast-720"
}
```

如果保存后仍然报“使用的模型不存在”，优先禁用再启用对应 NewAPI 渠道，或重启 NewAPI，避免旧配置缓存继续生效。
