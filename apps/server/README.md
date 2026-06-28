## Setup

### 永続化

NeoShowcase の MariaDB 環境変数がすべて存在する場合、OAuth セッションとチャンネルスコアを MariaDB に永続化します。

```bash
NS_MARIADB_DATABASE=...
NS_MARIADB_HOSTNAME=...
NS_MARIADB_PASSWORD=...
NS_MARIADB_PORT=...
NS_MARIADB_USER=...
```

未設定の場合は従来通りインメモリで動作します。一部だけ設定されている場合は起動に失敗します。

Cloud Run から Cloud SQL に接続する場合は、Cloud Run service に Cloud SQL instance を接続し、
`NS_MARIADB_HOSTNAME` に `/cloudsql/PROJECT:REGION:INSTANCE` を設定します。
この場合 `NS_MARIADB_PORT` は設定必須ですが、接続には使用されません。

## Tools

### ローカル実行

```bash
go run .
```

### 本番ビルド

```bash
go build -tags production .
```

### モッククライアント

####　起動

```bash
make mock-up
```

#### 停止

```bash
make mock-down
```
