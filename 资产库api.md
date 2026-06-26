# 资源管理API接口文档

## 通用说明

1. 所有接口必须携带有效的`token`认证头，示例中的token仅为演示，实际使用时请替换为真实有效的用户令牌

2. 所有接口的`Content-Type`必须设置为`application/json`

3. 响应状态码`code=0`表示请求成功，非0表示请求失败，具体错误信息请查看`message`字段

4. 删除接口支持两种模式：单个删除（传`Id`参数）和批量删除（传`Ids`数组参数）

5. api对接联系客服开通



# 一、资源分组管理

### 1\. 查询资源分组列表

- **请求方式**：POST

- **请求URL**：`http://119.45.42.208:8620/resources/user/ResourcesTypeList`

- **请求头**：

    |    字段名|    类型|    说明|
    |---|---|---|
    |    token|    string|    用户认证令牌|
    |    Content\-Type|    string|    固定值：`application/json`|

- **请求体**：

    ```JSON
    {
      "Page": 1
    }
    ```

    |    字段名|    类型|    说明|
    |---|---|---|
    |    Page|    int|    页码，从1开始|

- **响应示例**：

    ```JSON
    {
      "code": 0,
      "message": "请求成功",
      "data": {
        "data": [
          {
            "Id": 1,
            "CreatedAt": "2026-06-02T21:25:40+08:00",
            "UpdatedAt": "2026-06-02T21:25:40+08:00",
            "CreatedId": 20,
            "CreatedName": "",
            "Name": "默认分组"
          }
        ],
        "total": 2
      },
      "success": true
    }
    ```

### 2\. 添加资源分组

- **请求方式**：POST

- **请求URL**：`http://119.45.42.208:8620/resources/user/ResourcesType`

- **请求头**：

    |    字段名|    类型|    说明|
    |---|---|---|
    |    token|    string|    用户认证令牌|
    |    Content\-Type|    string|    固定值：`application/json`|

- **请求体**：

    ```JSON
    {
      "Name": "分类名"
    }
    ```

    |    字段名|    类型|    说明|
    |---|---|---|
    |    Name|    string|    分组名称|

- **响应示例**：

    ```JSON
    {
      "code": 0,
      "message": "请求成功",
      "data": {},
      "success": true
    }
    ```

### 3\. 删除资源分组

- **请求方式**：DELETE

- **请求URL**：`http://119.45.42.208:8620/resourcesapi/user/ResourcesType`

- **请求头**：

    |    字段名|    类型|    说明|
    |---|---|---|
    |    token|    string|    用户认证令牌|
    |    Content\-Type|    string|    固定值：`application/json`|

- **请求体**：

    ```JSON
    {
      "Id": 2
    }
    ```

    |    字段名|    类型|    说明|
    |---|---|---|
    |    Id|    int|    要删除的分组ID|

- **响应示例**：

    ```JSON
    {
      "code": 0,
      "message": "请求成功",
      "data": {},
      "success": true
    }
    ```

## 二、资源列表管理

### 1\. 查询资源列表

- **请求方式**：POST

- **请求URL**：`http://119.45.42.208:8620/resources/user/ResourcesList`

- **请求头**：

    |    字段名|    类型|    说明|
    |---|---|---|
    |    token|    string|    用户认证令牌|
    |    Content\-Type|    string|    固定值：`application/json`|

- **请求体**：

    ```JSON
    {
      "Page": 1
    }
    ```

    |    字段名|    类型|    说明|
    |---|---|---|
    |    Page|    int|    页码，从1开始|

- **响应示例**：

    ```JSON
    {
      "code": 0,
      "message": "请求成功",
      "data": {
        "data": [
          {
            "Id": 11,
            "CreatedAt": "2026-06-03T21:55:05+08:00",
            "UpdatedAt": "2026-06-03T21:55:05+08:00",
            "CreatedId": 20,
            "CreatedName": "",
            "UserUserId": 20,
            "UserResourcesGroupId": 0,
            "Name": "林春芽",
            "OssPath": "aaaaaa.png",
            "Desc": "",
            "Prompt": "",
            "UserResourcesTypeId": 2,
            "Status": 1,
            "StatusText": "处理成功",
            "Message": "",
            "AssetId": "asset-****"
          }
        ],
        "total": 11
      },
      "success": true
    }
    ```

    > **重要说明**：响应中存在`AssetId`字段表示该资源已加白成功
    > 
    > 

### 2\. 添加资源

- **请求方式**：POST

- **请求URL**：`http://119.45.42.208:8620/resources/user/Resources`

- **请求头**：

    |    字段名|    类型|    说明|
    |---|---|---|
    |    token|    string|    用户认证令牌|
    |    Content\-Type|    string|    固定值：`application/json`|

- **请求体**：

    ```JSON
    {
      "Name": "测试名",
      "OssPath": "https://aaaaaa.png"
    }
    ```

    |    字段名|    类型|    说明|
    |---|---|---|
    |    Name|    string|    资源名称|
    |    OssPath|    string|    可访问的图片地址|
    |    UserResourcesTypeId|    int|    资源分组id，不传默认分组|

- **响应示例**：

    ```JSON
    {
      "code": 0,
      "message": "请求成功",
      "data": {},
      "success": true
    }
    ```

### 3\. 删除资源

- **请求方式**：DELETE

- **请求URL**：`http://119.45.42.208:8620/resourcesapi/user/Resources`

- **请求头**：

    |    字段名|    类型|    说明|
    |---|---|---|
    |    token|    string|    用户认证令牌|
    |    Content\-Type|    string|    固定值：`application/json`|

- **请求体**：

    ```JSON
    {
      "Id": 1,
      "Ids": [1, 2]
    }
    ```

    |    字段名|    类型|    说明|
    |---|---|---|
    |    Id|    int|    单个资源ID（与Ids二选一）|
    |    Ids|    array|    批量资源ID列表（与Id二选一）|

- **响应示例**：

    ```JSON
    {
      "code": 0,
      "message": "请求成功",
      "data": {},
      "success": true
    }
    ```

## 



