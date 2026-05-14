# CloudFront Signed URL Demo（Go）

一个生成 **CloudFront Canned Policy 签名 URL** 的最小化 HTTP Server，用于限制 S3 静态资源的访问权限。  
仅使用 Go 标准库，无任何外部依赖。

---

## 工作原理

1. 客户端发送 `GET /sign?path=/images/photo.jpg`
2. Server 构造 [Canned Policy](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/private-content-creating-signed-url-canned-policy.html) JSON，包含资源 URL 和过期时间戳
3. Server 用 CloudFront 私钥对 Policy 进行 RSA-SHA1 签名
4. Server 返回签名后的 URL

---

## 前置准备

### 1. 生成 CloudFront 密钥对

CloudFront 要求使用 **2048 位 RSA** 密钥对。私钥保存在本地服务器，公钥上传到 AWS。

```bash
# 生成私钥（PKCS#1 PEM 格式，妥善保管，不要提交到代码仓库）
openssl genrsa -out cloudfront-private-key.pem 2048

# 提取公钥，用于上传到 AWS
openssl rsa -pubout -in cloudfront-private-key.pem -out cloudfront-public-key.pem
```

> `cloudfront-private-key.pem` 已加入 `.gitignore`，**切勿提交到版本控制**。

### 2. 在 AWS 控制台上传公钥

1. 打开 **AWS 控制台 → CloudFront → 密钥管理 → 公有密钥**
2. 点击**添加公有密钥**
3. 粘贴 `cloudfront-public-key.pem` 的内容
4. 记录生成的**密钥 ID**，填入配置文件的 `key_pair_id`

### 3. 创建密钥组

1. 进入 **CloudFront → 密钥管理 → 密钥组**
2. 点击**添加密钥组**，选择刚才创建的公有密钥
3. 在 CloudFront Distribution 的**缓存行为**中关联该密钥组：  
   编辑缓存行为 → 限制查看器访问 → 是 → 受信任的密钥组 → 选择你的密钥组

### 4. 配置 S3 存储桶

确保 S3 存储桶**不对公众开放**，通过 **Origin Access Control（OAC）** 让 CloudFront 访问：

1. **CloudFront → 源 → 编辑** S3 源
2. 将**源访问**设置为「源访问控制设置」
3. 创建或选择一个 OAC，按提示更新 S3 存储桶策略

---

## 配置文件

复制模板后编辑：

```bash
cp config.example.json config.json
```

```json
{
  "distribution_domain": "d1234abcdef8.cloudfront.net",
  "key_pair_id": "K1UA3WV15I7JSD",
  "private_key_path": "./cloudfront-private-key.pem",
  "default_ttl_seconds": 3600,
  "listen_addr": ":8080"
}
```

| 字段 | 说明 |
|---|---|
| `distribution_domain` | CloudFront Distribution 域名（不含 `https://`） |
| `key_pair_id` | CloudFront 公有密钥 ID |
| `private_key_path` | 私钥 PEM 文件路径 |
| `default_ttl_seconds` | 签名 URL 默认有效期（秒），默认 3600 |
| `listen_addr` | HTTP Server 监听地址，默认 `:8080` |

---

## 构建与运行

```bash
# 编译
go build -o server .

# 运行（默认读取 config.json）
./server

# 指定配置文件路径
./server -config /path/to/config.json
```

---

## API

本 demo 提供两个签名接口，逻辑等价，实现不同：

| 路径 | 实现 |
|---|---|
| `/sign` | 标准库实现（无外部依赖） |
| `/sign_sdk` | AWS SDK v2 实现（`feature/cloudfront/sign`） |

### `GET /sign` 和 `GET /sign_sdk`

为指定 S3 对象生成签名 URL。

**查询参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `path` | 是 | S3 对象路径，如 `/images/photo.jpg` |
| `ttl` | 否 | 本次请求的有效期（秒），不填则使用配置中 `default_ttl_seconds` 的值（默认 3600 秒） |

**请求示例**

```bash
# 标准库实现
curl "http://localhost:8080/sign?path=/images/photo.jpg"
curl "http://localhost:8080/sign?path=/videos/demo.mp4&ttl=300"

# SDK v2 实现
curl "http://localhost:8080/sign_sdk?path=/images/photo.jpg"
curl "http://localhost:8080/sign_sdk?path=/videos/demo.mp4&ttl=300"
```

**响应示例**

```json
{
  "path": "/images/photo.jpg",
  "signed_url": "https://d1234abcdef8.cloudfront.net/images/photo.jpg?Expires=1234567890&Signature=...&Key-Pair-Id=K1UA3WV15I7JSD"
}
```

### `GET /health`

```json
{"status": "ok"}
```

---

## 签名逻辑

实现遵循 [AWS CloudFront Canned Policy 规范](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/private-content-creating-signed-url-canned-policy.html)：

1. 构造 Policy 字符串（不含空格）：
   ```
   {"Statement":[{"Resource":"<url>","Condition":{"DateLessThan":{"AWS:EpochTime":<epoch>}}}]}
   ```
2. 使用 **RSA-SHA1**（PKCS#1 v1.5）对 Policy 签名
3. 对签名结果进行 Base64 编码，并替换 CloudFront 要求的特殊字符：`+→-`、`=→_`、`/→~`
4. 在 URL 后拼接 `?Expires=<epoch>&Signature=<sig>&Key-Pair-Id=<id>`
