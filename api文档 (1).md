# api文档

所有token，需要提供手机号给管理员，管理员处理好后给客户。



token计算方式

https://bytedance\.larkoffice\.com/share/base/form/shrcnP1Bl0mqCP9OHCbjpe1oBkf



# Seedance视频生成接口文档

## 接口基础信息

- **接口名称**：通用图片生成视频接口

- **请求地址**：`http://119.45.252.34:8618/seedanceapi/common/File/All`

- **请求方式**：`POST`

- **数据格式**：`multipart/form-data`

- **接口状态**：正式可用

## 请求请求头

|参数名|必传|类型|说明|
|---|---|---|---|
|token|是|string|接口授权密钥|



## 请求表单参数

|参数名|必传|类型|示例值|参数说明|
|---|---|---|---|---|
|model|是|string|doubao\-seedance\-2\-0\-fast\-260128|参考下表|
|prompt|是|string|让图片1站在图片2的舞台上讲话说同志们辛苦了|视频生成提示词|
|duration|是|string|5|视频时长，单位秒\(4\-15\)|
|aspect\_ratio|是|string|4:3|视频画面比例 21:9,16:9,4:3,1:1,3:4,9:16|
|files|是|file|本地图片文件|多张图片资源文件，支持多文件传参<br>可以使用图片链接地址，和资源库地址（asset://资产id）|
|generate\_audio|否|string|true|是否生成配音音频，true开启/false关闭|
|watermark|否|string|false|是否添加水印，true开启/false关闭|
|resolution|否|string|720p，480p，1080p|视频清晰度分辨率|

model选项

|模型名|模型说明|
|---|---|
|doubao\-seedance\-2\-0\-fast\-260128|快速版、9图、3视频、3音频、3\-5分钟、GF|
|doubao\-seedance\-2\-0\-260128|满血版、9图、3视频、3音频、5\-10分钟、GF|
|doubao\-seedance\-2\-0\-260128\-1|满血版、9图、3视频、3音频、20\-50分钟\(暂时用不了\)|
|doubao\-seedance\-2\-0\-260128\-2|快速版、9图、3音视频（每个大于4秒，总秒数小于15）、3\-8分钟|
|doubao\-seedance\-2\-0\-260128\-3|满血版、9图、3音视频（每个大于4秒，总秒数小于15）、8\-10分钟|



## 完整请求示例

### CURL 请求

```Bash
curl --location 'http://119.45.252.34:8618/seedanceapi/common/File/All' \
--header 'token: ****************' \
--form 'model="doubao-seedance-2-0-fast-260128"' \
--form 'prompt="让图片1站在图片2的舞台上讲话说同志们辛苦了"' \
--form 'duration="6"' \
--form 'aspect_ratio="4:3"' \
--form 'files=@"/Users/loken/Documents/aaaaaa.png"' \
--form 'files=@"/Users/loken/Documents/bbbbbb.png"' \
--form 'generate_audio="true"' \
--form 'watermark="false"' \
--form 'resolution="720p"'
```



## 接口返回数据

### 成功返回示例

```JSON
{
    "code": 0,
    "message": "请求成功",
    "data": {
        "Id": 18
    },
    "success": true
}
```



### 返回字段说明

|字段名|类型|说明|
|---|---|---|
|code|int|状态码，0=请求成功|
|message|string|响应提示信息|
|success|bool|请求业务状态|
|data\.Id|int|视频生成任务唯一ID，用于后续查询任务状态、获取视频链接|



---



# 视频任务查询接口文档

## 接口基础信息

- **接口名称**：视频生成任务详情查询

- **请求地址**：`http://119.45.252.34:8618/seedanceapi/user/DataIndex`

- **请求方式**：`POST`

- **数据格式**：`application/json`

- **接口状态**：正式可用



## 请求头信息

|参数名|必传|类型|说明|
|---|---|---|---|
|token|是|string|接口授权密钥|
|Content\-Type|是|string|固定值：application/json|





## 请求体参数

|参数名|必传|类型|示例值|参数说明|
|---|---|---|---|---|
|Id|是|int|18|视频生成任务ID（来自提交生成接口返回的data\.Id）|





## 完整请求示例

### CURL 请求

```Bash
curl --location 'http://119.45.252.34:8618/seedanceapi/user/DataIndex' \
--header 'token: ************' \
--header 'Content-Type: application/json' \
--data '{"Id":18}'
```



## 接口返回数据

### 成功返回示例

```JSON
{
    "code": 0,
    "message": "请求成功",
    "data": {
        "Id": 18,
        "CreatedAt": "2026-05-20T20:50:58+08:00",
        "UpdatedAt": "2026-05-20T20:50:58+08:00",
        "Status": 0,
        "StatusText": "待处理",
        "Message": "",
        "VideoUrl": "",
        "UseToken":0,
        "DeductToken":0,
        "UseDuration":0
        
    },
    "success": true
}
```



### 返回字段说明

|字段名|类型|说明|
|---|---|---|
|code|int|状态码，0=请求成功|
|message|string|响应提示信息|
|success|bool|请求业务状态|
|data\.Id|int|任务唯一ID|
|data\.CreatedAt|string|任务创建时间|
|data\.UpdatedAt|string|任务最后更新时间|
|data\.Status|int|任务状态码|
|data\.StatusText|string|任务状态文案|
|data\.Message|string|任务异常信息（正常为空）|
|data\.VideoUrl|string|视频播放/下载地址（生成完成后返回）|
|data\.UseToken|int|消耗的token|
|data\.DeductToken|int|0:待扣\|1:扣成功\|2:扣失败|
|data\.UseDuration|int|消耗的秒数|



### 任务状态说明

|Status|StatusText|说明|
|---|---|---|
|0|待处理|任务已提交，排队中|
|1|处理中|正在生成视频|
|2|已完成|视频生成成功，VideoUrl有值|
|3|失败|生成失败，Message显示原因|



## 业务使用说明

1. **配合生成接口使用**：先调用**视频生成接口**获取任务ID，再通过此接口查询进度

2. **轮询建议**：建议每隔**30秒**调用一次，直到Status=2（完成）或Status=3（失败）

3. **结果获取**：当`Status=2`时，`VideoUrl`为最终视频地址，可直接播放/下载

4. **异常处理**：当`Status=`3时，`Message`字段会返回失败原因（如图片违规、参数错误等）

---



## 调用注意事项

1. 必须传入正确的**任务ID**，否则会查询失败

2. VideoUrl为临时地址，建议生成后尽快下载保存

3. token必须与提交生成任务时保持一致

4. 视频有效期24小时，请尽快保存到本地







# 任务列表查询接口文档

## 接口基础信息

- **接口名称**：视频生成任务列表查询

- **请求地址**：`http://119.45.252.34:8618/seedanceapi/user/DataList`

- **请求方式**：`POST`

- **数据格式**：`application/json`

- **接口状态**：正式可用

## 请求头

|参数名|必传|类型|说明|
|---|---|---|---|
|token|是|string|接口授权密钥|
|Content\-Type|是|string|固定值：`application/json`|



**请求头示例**

```Plain Text
token: ****
Content-Type: application/json
```



## 请求体参数

|参数名|必传|类型|示例值|说明|
|---|---|---|---|---|
|Page|是|int|1|页码，从 1 开始|
||||||



## 完整请求示例

```Bash
curl --location 'http://119.45.252.34:8618/seedanceapi/user/DataList' \
--header 'token: ****' \
--header 'Content-Type: application/json' \
--data '{"Page":1}'
```



## 返回数据说明

### 成功返回示例

```JSON
{
    "code": 0,
    "message": "请求成功",
    "data": {
        "data": [
            {
                "Id": 1,
                "CreatedAt": "2026-05-20T23:19:21+08:00",
                "UpdatedAt": "2026-05-20T23:19:21+08:00",
                "CreatedId": 49,
                "CreatedName": "",
                "UserUserId": 0,
                "Data": "",
                "Taskid": "",
                "Status": 3,
                "StatusText": "处理失败",
                "Message": "token不足",
                "Pormat": "让图片1站在图片2的舞台上讲话说同志们辛苦了",
                "VideoUrl": "",
                "UseToken": 0,
                "DeductToken": 0,
                "DeductTokenText": "待扣",
                "UseDuration": 0
            }
        ],
        "total": 1
    },
    "success": true
}
```



### 顶层返回字段

|字段名|类型|说明|
|---|---|---|
|code|int|状态码，`0` 表示请求成功|
|message|string|响应描述信息|
|success|bool|接口请求结果标识|
|data|object|分页数据主体|
|data\.total|int|数据总条数|
|data\.data|array|任务列表数组|



### 列表单项字段

|字段名|类型|说明|
|---|---|---|
|Id|int|任务唯一ID|
|CreatedAt|string|任务创建时间|
|UpdatedAt|string|任务最后更新时间|
|CreatedId|int|创建人ID|
|CreatedName|string|创建人名称|
|UserUserId|int|关联用户ID|
|Taskid|string|底层任务标识|
|Status|int|任务状态码|
|StatusText|string|任务状态文案|
|Message|string|任务备注/失败原因|
|Pormat|string|生成视频使用的提示词|
|VideoUrl|string|视频地址，生成成功后返回|
|UseToken|int|本次消耗额度|
|DeductToken|int|扣除状态|
|DeductTokenText|string|扣除状态描述|
|UseDuration|int|视频时长（单位：秒）|



### 任务状态参考

|Status|StatusText|说明|
|---|---|---|
|0|待处理|任务排队中|
|1|处理中|视频生成中|
|2|已完成|生成成功|
|3|处理失败|任务执行失败|



## 使用说明

1. 该接口用于分页查询当前账号下所有历史视频生成任务。

2. 分页仅需传入页码 `Page`，默认按创建时间倒序返回数据。

3. 可通过 `Status`、`Message` 判断任务执行结果与失败原因。

4. 已完成任务可通过 `VideoUrl` 获取视频地址，`Pormat` 可查看原始提示词。

## 调用注意事项

1. 授权 `token` 需和业务接口保持一致，否则查询无数据或鉴权失败。

2. 列表数据为历史任务记录，建议按需分页拉取。

3. `Message` 字段在任务失败时会展示具体失败原因，可用于问题排查。









# 用户信息接口文档

## 接口概述

- **接口名称**：用户首页信息查询

- **接口地址**：`http://119.45.252.34:8618/seedanceapi/user/UserIndex`

- **请求方式**：`POST`

- **Content\-Type**：`application/json`

- **接口描述**：根据用户ID查询用户首页基础信息，需携带有效token鉴权

## 请求参数

### 请求头（Header）

|参数名|必选|类型|说明|
|---|---|---|---|
|token|是|string|用户身份鉴权令牌|
|Content\-Type|是|string|固定值：application/json|



### 请求体（Body）

**参数格式**：JSON

|参数名|必选|类型|说明|
|---|---|---|---|
|Id|是|int|用户ID（默认传0即可）|



### 请求示例

```Bash
curl --location 'http://119.45.252.34:8618/seedanceapi/user/UserIndex' \
--header 'token: ' \
--header 'Content-Type: application/json' \
--data '{"Id":0}'
```



## 响应参数

### 响应体结构

|字段名|类型|说明|
|---|---|---|
|code|int|响应状态码：0=成功|
|message|string|响应描述|
|data|object|业务数据实体|
|success|bool|请求是否成功：true=成功|



### data 字段详情

|字段名|类型|说明|
|---|---|---|
|Id|int|用户唯一标识|
|CreatedAt|string|创建时间|
|UpdatedAt|string|更新时间|
|CreatedId|int|创建人ID|
|CreatedName|string|创建人名称|
|Phone|string|用户手机号|
|Token|int|国内满血版剩余token|
|FastToken|int|国内快速版剩余token|
|SdDuration|int|国际版剩余时长（单位：秒）|



### 成功响应示例

```JSON
{
    "code": 0,
    "message": "请求成功",
    "data": {
        "Id": 6,
        "CreatedAt": "",
        "UpdatedAt": "",
        "CreatedId": 0,
        "CreatedName": "",
        "Phone": "",
        "Token": 36300,
        "FastToken": 937500,
        "SdDuration": 36
    },
    "success": true
}
```



