# Broth -- データアクセス層・ORM・管理画面設計書

> **バージョン**: 0.1.0-draft
> **最終更新**: 2026-02-08
> **ステータス**: 初期設計
> **前提ドキュメント**: [ARCHITECTURE.md](./ARCHITECTURE.md) / [MODULE_DESIGN.md](./MODULE_DESIGN.md)


---

## 目次

1. [データアクセス層の設計方針](#1-データアクセス層の設計方針)
2. [クエリビルダ / コード生成の設計](#2-クエリビルダ--コード生成の設計)
3. [トランザクション管理](#3-トランザクション管理)
4. [マイグレーション設計](#4-マイグレーション設計)
5. [管理画面（Admin）の構想](#5-管理画面adminの構想)
6. [テンプレートエンジン](#6-テンプレートエンジン)
7. [機能比較表](#7-機能比較表)

---

## 1. データアクセス層の設計方針

### 1.1 基本方針: database/sql を基盤にする

Broth のデータアクセス層は `database/sql` を唯一の基盤とする。GORM のような重量級 ORM を採用せず、SQL を第一級市民として扱う。

**設計判断の根拠**:

| 観点 | database/sql 直接利用 | GORM 等の ORM |
|---|---|---|
| **型安全性** | コード生成で保証 | リフレクション依存、実行時エラーのリスク |
| **SQL の可視性** | SQL がそのまま見える | メソッドチェーンの裏で何が発行されているか不透明 |
| **パフォーマンス** | SQL チューニングが直接可能 | ORM が生成する SQL の制御が困難 |
| **学習コスト** | SQL + Go の知識で十分 | ORM 固有の API を覚える必要がある |
| **Go らしさ** | 標準ライブラリの範囲 | リフレクション多用は Go イディオムに反する |
| **デバッグ** | SQL が明示的で追跡容易 | N+1 問題が暗黙的に発生しうる |

> **「DB スキーマから型安全な Go コードを生成」** のアプローチは sqlc・SQLBoiler が実証し、Bob がさらに洗練した。Broth は Bob をベースにこの方向性をフレームワークに統合する。

### 1.2 broth/db パッケージの責務

`broth/db` はデータアクセス層の基盤パッケージである。責務を明確に限定する。

```mermaid
graph TB
    subgraph "broth/db パッケージの責務"
        CONN["<b>コネクション管理</b><br/>Open / Close / Health Check<br/>コネクションプール設定"]
        TX["<b>トランザクション管理</b><br/>TxManager / RunInTx<br/>コンテキスト経由の伝搬"]
        CTX["<b>コンテキストブリッジ</b><br/>ConnFromContext<br/>*sql.DB / *sql.Tx の透過的切替"]
        INST["<b>計装 (Instrumentation)</b><br/>クエリログ / トレーシング<br/>スロークエリ検出"]
    end

    CONN --> TX
    TX --> CTX
    CONN --> INST

    style CONN fill:#4A90D9,color:#fff
    style TX fill:#E8724A,color:#fff
    style CTX fill:#50B848,color:#fff
    style INST fill:#9B59B6,color:#fff
```

**broth/db が担わないもの**:
- SQL の組み立て（クエリビルダは `broth/query` パッケージ）
- マイグレーション（`broth/migrate` パッケージ）
- ORM 的なモデルマッピング（コード生成で解決）

### 1.3 Database 構造体

```go
// broth/db/db.go
package db

import (
    "context"
    "database/sql"
    "fmt"
    "log/slog"
    "time"
)

// Config はデータベース接続設定を保持する。
type Config struct {
    URL             string        // "postgres://user:pass@host:5432/dbname?sslmode=disable"
    MaxOpenConns    int           // 最大接続数（デフォルト: 25）
    MaxIdleConns    int           // 最大アイドル接続数（デフォルト: 5）
    ConnMaxLifetime time.Duration // 接続の最大生存時間（デフォルト: 30m）
    ConnMaxIdleTime time.Duration // アイドル接続の最大生存時間（デフォルト: 5m）
}

// DefaultConfig はデフォルトのコネクションプール設定を返す。
// 7+2人チーム規模のアプリケーションに適した値を設定している。
func DefaultConfig(url string) Config {
    return Config{
        URL:             url,
        MaxOpenConns:    25,
        MaxIdleConns:    5,
        ConnMaxLifetime: 30 * time.Minute,
        ConnMaxIdleTime: 5 * time.Minute,
    }
}

// Database はデータベース接続を管理する。
// アプリケーション全体で1インスタンスを共有する。
type Database struct {
    db     *sql.DB
    logger *slog.Logger
}

// Open はデータベース接続を確立する。
func Open(cfg Config, logger *slog.Logger) (*Database, error) {
    sqlDB, err := sql.Open("postgres", cfg.URL)
    if err != nil {
        return nil, fmt.Errorf("db: open: %w", err)
    }

    // コネクションプール設定
    sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
    sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
    sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
    sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

    // 接続確認
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := sqlDB.PingContext(ctx); err != nil {
        return nil, fmt.Errorf("db: ping: %w", err)
    }

    logger.Info("database connected",
        "max_open_conns", cfg.MaxOpenConns,
        "max_idle_conns", cfg.MaxIdleConns,
    )

    return &Database{db: sqlDB, logger: logger}, nil
}

// MustOpen は Open のパニック版。main.go での初期化用。
func MustOpen(cfg Config, logger *slog.Logger) *Database {
    d, err := Open(cfg, logger)
    if err != nil {
        panic(fmt.Sprintf("db: must open: %v", err))
    }
    return d
}

// DB は内部の *sql.DB を返す。リポジトリ実装に渡す。
func (d *Database) DB() *sql.DB { return d.db }

// TxManager はトランザクションマネージャを返す。
func (d *Database) TxManager() *TxMgr {
    return NewTxManager(d.db, d.logger)
}

// Close はデータベース接続を閉じる。
func (d *Database) Close() error { return d.db.Close() }

// HealthCheck はデータベースの死活を確認する。
func (d *Database) HealthCheck(ctx context.Context) error {
    return d.db.PingContext(ctx)
}
```

### 1.4 コネクションプール設定の標準化

| パラメータ | デフォルト | 根拠 |
|---|---|---|
| `MaxOpenConns` | 25 | PostgreSQL デフォルト max_connections=100。アプリ複数台でシェアする想定 |
| `MaxIdleConns` | 5 | 通常負荷時の接続維持。不要な接続保持を避ける |
| `ConnMaxLifetime` | 30m | DB 側のタイムアウトやロードバランサとの整合性 |
| `ConnMaxIdleTime` | 5m | アイドル接続の早期解放でリソースを節約 |

これらの値は `broth/config` 経由で環境変数からオーバーライド可能とする。

```
BROTH_DB_MAX_OPEN_CONNS=50
BROTH_DB_MAX_IDLE_CONNS=10
BROTH_DB_CONN_MAX_LIFETIME=1h
BROTH_DB_CONN_MAX_IDLE_TIME=10m
```

### 1.5 Repository パターンの標準形

ARCHITECTURE.md / MODULE_DESIGN.md で定めた Repository パターンをデータアクセス層の観点から整理する。

```mermaid
graph LR
    subgraph "modules/account/ (公開)"
        REPO_IF["<b>repository.go</b><br/>Repository interface"]
        MODEL["<b>model.go</b><br/>User 構造体"]
    end

    subgraph "modules/account/internal/store/ (非公開)"
        REPO_IMPL["<b>user.go</b><br/>UserStore 実装"]
        GEN_CODE["<b>queries.broth.go</b><br/>生成コード (go generate)"]
    end

    subgraph "broth/db"
        DB_PKG["Database / TxManager<br/>ConnFromContext"]
    end

    REPO_IMPL -.->|implements| REPO_IF
    REPO_IMPL --> MODEL
    REPO_IMPL --> DB_PKG
    REPO_IMPL --> GEN_CODE

    style REPO_IF fill:#9B59B6,color:#fff
    style MODEL fill:#50B848,color:#fff
    style REPO_IMPL fill:#9B59B6,color:#fff
    style GEN_CODE fill:#F5A623,color:#fff
    style DB_PKG fill:#4A90D9,color:#fff
```

#### インターフェース定義（公開パッケージ）

```go
// modules/account/repository.go
package account

import (
    "context"

    "myapp/modules/shared"
)

// Repository はユーザーデータへのアクセスを抽象化する。
// メソッドはビジネスユースケースに対応する粒度で定義する。
// CRUD の全メソッドを機械的に定義するのではなく、
// 実際に必要な操作のみを宣言する。
type Repository interface {
    // Create はユーザーを永続化する。ID は採番されて user.ID に設定される。
    Create(ctx context.Context, user *User) error

    // FindByID は ID でユーザーを検索する。見つからない場合は nil, nil を返す。
    FindByID(ctx context.Context, id int64) (*User, error)

    // FindByEmail はメールアドレスでユーザーを検索する。
    FindByEmail(ctx context.Context, email string) (*User, error)

    // List はページネーション付きでユーザー一覧を取得する。
    List(ctx context.Context, page shared.Page, filter UserFilter) (*shared.PageResult[User], error)

    // Update はユーザー情報を更新する。
    Update(ctx context.Context, user *User) error

    // Delete はユーザーを論理削除する。
    Delete(ctx context.Context, id int64) error
}

// UserFilter はユーザー検索のフィルタ条件。
type UserFilter struct {
    NameContains  string // 名前の部分一致
    EmailContains string // メールアドレスの部分一致
    IsActive      *bool  // アクティブ状態でフィルタ（nil=条件なし）
}
```

#### 実装（internal/store 配下）

```go
// modules/account/internal/store/user.go
package store

import (
    "context"
    "database/sql"
    "fmt"

    "myapp/modules/account"
    "myapp/modules/shared"
    "myapp/broth/db"
)

// UserStore は account.Repository の PostgreSQL 実装。
type UserStore struct {
    pool *sql.DB
}

// New は UserStore を生成する。
func New(pool *sql.DB) *UserStore {
    return &UserStore{pool: pool}
}

// コンパイル時にインターフェース適合を検証する。
var _ account.Repository = (*UserStore)(nil)

// Create はユーザーをDBに挿入する。
func (s *UserStore) Create(ctx context.Context, user *account.User) error {
    conn := db.ConnFromContext(ctx, s.pool)
    const q = `
        INSERT INTO users (email, name, password_hash, is_active, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id`
    return conn.QueryRowContext(ctx, q,
        user.Email, user.Name, user.PasswordHash,
        user.IsActive, user.CreatedAt, user.UpdatedAt,
    ).Scan(&user.ID)
}

// FindByID は ID でユーザーを検索する。
func (s *UserStore) FindByID(ctx context.Context, id int64) (*account.User, error) {
    conn := db.ConnFromContext(ctx, s.pool)
    const q = `
        SELECT id, email, name, password_hash, is_active, created_at, updated_at
        FROM users
        WHERE id = $1 AND deleted_at IS NULL`
    return scanUser(conn.QueryRowContext(ctx, q, id))
}

// List はページネーション付きでユーザー一覧を取得する。
func (s *UserStore) List(ctx context.Context, page shared.Page, filter account.UserFilter) (*shared.PageResult[account.User], error) {
    conn := db.ConnFromContext(ctx, s.pool)

    // 動的クエリ組み立て（broth/query パッケージを使用）
    where, args := buildUserFilter(filter)

    // カウントクエリ
    countQ := fmt.Sprintf("SELECT COUNT(*) FROM users WHERE deleted_at IS NULL %s", where)
    var total int64
    if err := conn.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
        return nil, fmt.Errorf("store: count users: %w", err)
    }

    // データクエリ
    offset := (page.Number - 1) * page.Size
    dataQ := fmt.Sprintf(`
        SELECT id, email, name, password_hash, is_active, created_at, updated_at
        FROM users
        WHERE deleted_at IS NULL %s
        ORDER BY created_at DESC
        LIMIT $%d OFFSET $%d`,
        where, len(args)+1, len(args)+2)
    args = append(args, page.Size, offset)

    rows, err := conn.QueryContext(ctx, dataQ, args...)
    if err != nil {
        return nil, fmt.Errorf("store: list users: %w", err)
    }
    defer rows.Close()

    var users []account.User
    for rows.Next() {
        u, err := scanUserFromRows(rows)
        if err != nil {
            return nil, err
        }
        users = append(users, *u)
    }

    return &shared.PageResult[account.User]{
        Items:      users,
        TotalCount: total,
        Page:       page,
    }, nil
}

// scanUser は単一行から User をスキャンするヘルパー。
func scanUser(row *sql.Row) (*account.User, error) {
    u := &account.User{}
    err := row.Scan(
        &u.ID, &u.Email, &u.Name, &u.PasswordHash,
        &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, fmt.Errorf("store: scan user: %w", err)
    }
    return u, nil
}
```

### 1.6 Conn インターフェース: *sql.DB と *sql.Tx の抽象化

トランザクションの有無をリポジトリ実装から透過的にするために、共通インターフェースを定義する。

```go
// broth/db/conn.go
package db

import (
    "context"
    "database/sql"
)

// Conn は *sql.DB と *sql.Tx の共通操作を抽象化する。
// リポジトリ実装はこのインターフェースを通じて SQL を発行する。
type Conn interface {
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// コンパイル時検証: *sql.DB と *sql.Tx が Conn を満たすことを保証。
var (
    _ Conn = (*sql.DB)(nil)
    _ Conn = (*sql.Tx)(nil)
)

// txKey はコンテキストにトランザクションを格納するキー。
type txKey struct{}

// ConnFromContext はコンテキストからトランザクションを取得する。
// トランザクション内であれば *sql.Tx を、そうでなければ fallback (*sql.DB) を返す。
// これにより、リポジトリ実装はトランザクションの有無を意識せずに SQL を発行できる。
func ConnFromContext(ctx context.Context, fallback *sql.DB) Conn {
    if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
        return tx
    }
    return fallback
}
```

---

## 2. データアクセスライブラリの設計

### 2.1 設計思想: Database-First コード生成 with Bob

Broth のデータアクセス戦略は **Bob**（`github.com/stephenafamo/bob`）を推奨ライブラリとして採用する。Bob は SQLBoiler のメンテナーが設計し直した後継プロジェクトであり、Broth の設計原則（型安全・リフレクション最小・`interface{}` 排除・コード生成優先）とほぼ完全に一致する。

```mermaid
graph LR
    SCHEMA["DB スキーマ<br/>(マイグレーション適用済み)"] -->|"broth generate model"| BOBGEN["bobgen-psql<br/>(コード生成)"]
    BOBGEN --> MODELS["models/<br/>(型安全なモデル + クエリメソッド)"]
    BOBGEN --> FACTORY["factory/<br/>(テスト用ファクトリー)"]
    SQL["手書き .sql<br/>(複雑なクエリ)"] -->|"bobgen-sql<br/>または sqlc"| CUSTOM["カスタムクエリ<br/>(型安全)"]

    style SCHEMA fill:#50B848,color:#fff
    style BOBGEN fill:#E8724A,color:#fff
    style MODELS fill:#4A90D9,color:#fff
    style FACTORY fill:#9B59B6,color:#fff
    style SQL fill:#888,color:#fff
    style CUSTOM fill:#888,color:#fff
```

### 2.2 なぜ Bob を選定したか

**検討経緯**: GORM → SQLBoiler → sqlc → Bob の順に評価。

| 観点 | GORM | sqlc | SQLBoiler | **Bob** |
|---|---|---|---|---|
| **アプローチ** | コードファースト ORM | SQL → Go 生成 | DB → Go 生成 | **DB → Go 生成** |
| **型安全性** | 実行時（リフレクション） | コンパイル時 | コンパイル時 | **コンパイル時** |
| **`interface{}` 排除** | **不合格** -- 全公開メソッドが `interface{}` | 合格 | 合格 | **合格 -- 公式に掲げている** |
| **リフレクション** | 多用 | なし | 最小限 | **なし（コード生成ベース）** |
| **リレーション** | Preload (簡便) | **弱い** -- JOIN → フラット構造体 | Eager Loading | **Eager Loading + LEFT JOIN Preloading** |
| **動的クエリ** | Where chain | 苦手 | Query Mod | **型安全な Query Mod（クエリタイプごとに独立）** |
| **テスト支援** | なし | なし | 自動テスト（品質に課題） | **ファクトリー生成（factory_bot インスパイア）** |
| **Go イディオム** | 逸脱 | 最高 | 良好 | **「Convenience, not magic」** |
| **メンテナンス** | 活発 | 活発 | 停滞 | **活発（SQLBoiler メンテナーが開発）** |
| **Broth 設計原則との整合** | **不合格** | 高い | 高い | **最も高い** |

**不採用の判断**:
- **GORM**: `interface{}` の氾濫、リフレクション多用、暗黙の振る舞い（ソフトデリート等）が Broth の設計原則と根本的に衝突（ADR-D007）
- **sqlc**: 設計原則との整合は最高だが、JOIN → フラット構造体のマッピングコストが実務で重い。リレーション操作の生産性が不足
- **SQLBoiler**: 思想的に最も近いが、メンテナンス体制が不安定。後継の Bob が存在する以上、旧版を採用する理由がない

### 2.3 Bob の設計原則と Broth の整合

| Broth 設計原則 | Bob の対応 |
|---|---|
| `interface{}` を公開 API に使わない | 公式に「No need for `interface{}`」を掲げる |
| リフレクションは最小限 | コード生成ベース。リフレクション依存なし |
| Database-First | DB スキーマからコード生成（マイグレーション SQL が真のソース） |
| Go イディオム | 「Convenience, not magic」が設計原則 |
| 型安全性 | 生成コードは全て型安全。Query Mod もクエリタイプごとに型制約 |
| コンストラクタ注入 | `bob.Executor` インターフェースで `*sql.DB` / `*sql.Tx` を受け入れ |
| SQL ファイルベースのマイグレーション | マイグレーションは外部ツールに委任（Broth の goose ラップ方針と合致） |

### 2.4 broth generate model コマンド

```
$ broth generate model [--module <name>]

# 例: account モジュールのモデルを生成
$ broth generate model --module account

# 全モジュールのモデルを生成
$ broth generate model
```

**処理フロー**:

```mermaid
sequenceDiagram
    participant Dev as 開発者
    participant CLI as broth generate model
    participant DB as PostgreSQL
    participant Bob as bobgen-psql

    Dev->>CLI: broth generate model --module account
    CLI->>DB: スキーマ情報を取得（テーブル、カラム、FK、インデックス）
    DB-->>CLI: スキーマメタデータ
    CLI->>Bob: bobgen-psql でコード生成
    Note over Bob: ・モデル構造体<br/>・型安全な Query Mod<br/>・Eager Loading メソッド<br/>・テスト用ファクトリー
    Bob-->>CLI: 生成された Go コード
    CLI->>CLI: Broth 規約に基づく配置（internal/store/）
    CLI-->>Dev: Generated models for module 'account' (3 tables)
```

### 2.5 生成コードの配置とディレクトリ構造

```
modules/account/
├── model.go                          # ドメインモデル（手書き）
├── repository.go                     # リポジトリインターフェース（手書き）
├── service.go                        # ビジネスロジック（手書き）
├── handler.go                        # HTTP ハンドラ（手書き）
├── queries/                          # 手書き SQL（複雑なクエリ用、任意）
│   └── user_stats.sql                # Bob の Query Mod で表現しにくいクエリ
├── internal/
│   └── store/
│       ├── bob_models.broth.go         # Bob 生成: テーブルモデル（編集禁止）
│       ├── bob_factory.broth.go        # Bob 生成: テスト用ファクトリー（編集禁止）
│       ├── user.go                   # 手書き: リポジトリ実装
│       └── generate.go              # go:generate ディレクティブ
└── ...
```

### 2.6 Bob を使った CRUD コード例

```go
// modules/account/internal/store/user.go（手書き）
package store

import (
    "context"
    "database/sql"

    "myapp/modules/account"
    "myapp/modules/account/internal/store/models" // Bob 生成
    "myapp/broth/db"

    "github.com/stephenafamo/bob/dialect/psql"
)

// UserStore は account.Repository の実装。
type UserStore struct {
    pool *sql.DB
}

func New(pool *sql.DB) *UserStore {
    return &UserStore{pool: pool}
}

var _ account.Repository = (*UserStore)(nil)

// FindByID は ID でユーザーを取得する。
func (s *UserStore) FindByID(ctx context.Context, id int64) (*account.User, error) {
    conn := db.ConnFromContext(ctx, s.pool)
    user, err := models.FindUser(ctx, conn, id)
    if err != nil {
        return nil, err
    }
    return toUser(user), nil
}

// List はフィルタ付きでユーザー一覧を取得する。
// Bob の型安全な Query Mod で動的条件を組み立てる。
func (s *UserStore) List(ctx context.Context, filter account.UserFilter) ([]*account.User, error) {
    conn := db.ConnFromContext(ctx, s.pool)

    // 型安全な条件組み立て -- 文字列ベースの Where ではない
    queryMods := []bob.Mod[*psql.SelectQuery]{
        models.SelectWhere.Users.DeletedAt.IsNull(),
    }
    if filter.NameContains != "" {
        queryMods = append(queryMods,
            models.SelectWhere.Users.Name.Like("%"+filter.NameContains+"%"),
        )
    }
    if filter.IsActive != nil {
        queryMods = append(queryMods,
            models.SelectWhere.Users.IsActive.EQ(*filter.IsActive),
        )
    }

    users, err := models.Users.Query(queryMods...).All(ctx, conn)
    if err != nil {
        return nil, err
    }
    return toUsers(users), nil
}

// Create はユーザーを永続化する。
func (s *UserStore) Create(ctx context.Context, user *account.User) error {
    conn := db.ConnFromContext(ctx, s.pool)
    inserted, err := models.Users.Insert(ctx, conn, &models.UserSetter{
        Email:        omit.From(user.Email),
        Name:         omit.From(user.Name),
        PasswordHash: omit.From(user.PasswordHash),
        IsActive:     omit.From(user.IsActive),
    })
    if err != nil {
        return err
    }
    user.ID = inserted.ID
    return nil
}
```

### 2.7 リレーション（Eager Loading）

Bob の最大の利点の一つが、型安全な Eager Loading によるリレーション解決である。sqlc で痛点だった「JOIN → フラット構造体のマッピングコスト」が解消される。

```go
// ユーザーと所属する会社を一括取得（N+1 問題なし）
users, err := models.Users.Query(
    models.SelectWhere.Users.CompanyID.EQ(companyID),
    models.PreloadUserCompany(),          // 型安全な Eager Loading
).All(ctx, conn)

for _, u := range users {
    fmt.Println(u.Name)
    fmt.Println(u.R.Company.Name)         // リレーション先に直接アクセス
}
```

```go
// 多段リレーション: Company → Users → Posts
users, err := models.Users.Query(
    models.SelectWhere.Users.CompanyID.EQ(companyID),
    models.PreloadUserCompany(),
    models.PreloadUserPosts(),            // 投稿も一括ロード
).All(ctx, conn)

for _, u := range users {
    fmt.Printf("%s (%s): %d posts\n", u.Name, u.R.Company.Name, len(u.R.Posts))
}
```

### 2.8 テスト用ファクトリー

Bob は Ruby の `factory_bot` にインスパイアされたテスト用ファクトリーを自動生成する。FK 制約に従い、依存するテーブルのレコードも自動的に作成される。

```go
// テスト例: ファクトリーで依存テーブルも自動作成
func TestListUsers(t *testing.T) {
    db := testutil.SetupTestDB(t)
    f := factory.New(db)

    // Users が依存する Company も自動的に生成される
    users, err := f.NewUser().CreateMany(ctx, 5)

    // テスト対象のリポジトリ
    store := New(db)
    result, err := store.List(ctx, account.UserFilter{})
    assert(t, len(result) == 5)
}
```

### 2.9 sqlc の補助的位置づけ

Bob の Query Mod で表現しにくい複雑なクエリ（集計、ウィンドウ関数、CTE 等）については、**sqlc を補助的に利用**できる。Bob 自体にも `bobgen-sql`（手書き SQL からのコード生成）機能があり、Phase 1 ではこちらを優先検証する。

```
推奨の優先順位:
1. Bob の生成コード + Query Mod（標準的な CRUD・リレーション）
2. Bob の bobgen-sql（複雑な手書き SQL のコード生成）
3. sqlc（bobgen-sql で不足する場合の補助）
4. database/sql 直書き（常にエスケープハッチとして利用可能）
```

### 2.10 go generate との統合

```go
// modules/account/internal/store/generate.go
package store

//go:generate broth generate model --module account
```

これにより `go generate ./...` で全モジュールのモデルコードが一括生成される。

### 2.11 Bob 採用のリスク管理

| リスク | 対策 |
|---|---|
| **v1 未到達（v0.30 台）** | Broth 自体も pre-v1 のため、依存先の API 変更は許容できるフェーズ |
| **コアメンテナー 1 人** | SQLBoiler と同構造だが、Bob の方が活発。最悪フォークに切り替え可能 |
| **コミュニティ規模が小さい** | Broth が Bob を採用しコントリビュートすることで、相互にエコシステムを強化 |
| **GORM との DX 差** | Bob は GORM に近い生産性（Eager Loading、型安全クエリ）を、コード生成ベースで提供 |

---

## 3. トランザクション管理

### 3.1 設計原則

| 原則 | 説明 |
|---|---|
| **トランザクション境界は Service 層が管理する** | Repository 層はトランザクションの開始/終了を知らない |
| **コンテキスト経由で伝搬する** | `*sql.Tx` を引数で渡さず、`context.Context` に格納する |
| **リポジトリは透過的に動作する** | `ConnFromContext` でトランザクション有無を意識しない |
| **ネストは Savepoint で対応する** | `RunInTx` のネスト呼び出しは Savepoint に自動変換する |

### 3.2 TxManager 実装

```go
// broth/db/tx.go
package db

import (
    "context"
    "database/sql"
    "fmt"
    "log/slog"
    "sync/atomic"
)

// TxManager はトランザクションのライフサイクルを管理するインターフェース。
type TxManager interface {
    // RunInTx はトランザクション内で fn を実行する。
    // fn が error を返した場合はロールバック、nil の場合はコミットする。
    // 既にトランザクション内の場合は Savepoint を使用する。
    RunInTx(ctx context.Context, fn func(ctx context.Context) error) error

    // RunInTxWithOpts は分離レベル等のオプション付きでトランザクションを実行する。
    RunInTxWithOpts(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context) error) error
}

// TxMgr は TxManager の実装。
type TxMgr struct {
    db     *sql.DB
    logger *slog.Logger
    spID   atomic.Int64 // Savepoint の一意ID生成用
}

// NewTxManager は TxMgr を生成する。
func NewTxManager(db *sql.DB, logger *slog.Logger) *TxMgr {
    return &TxMgr{db: db, logger: logger}
}

// RunInTx はトランザクション内で fn を実行する。
func (m *TxMgr) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
    return m.RunInTxWithOpts(ctx, nil, fn)
}

// RunInTxWithOpts は分離レベル等のオプション付きでトランザクションを実行する。
func (m *TxMgr) RunInTxWithOpts(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context) error) error {
    // 既にトランザクション内の場合は Savepoint を使う
    if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
        return m.runInSavepoint(ctx, tx, fn)
    }

    // 新規トランザクション開始
    tx, err := m.db.BeginTx(ctx, opts)
    if err != nil {
        return fmt.Errorf("db: begin tx: %w", err)
    }

    // トランザクションをコンテキストに格納
    txCtx := context.WithValue(ctx, txKey{}, tx)

    // fn の実行
    if err := fn(txCtx); err != nil {
        if rbErr := tx.Rollback(); rbErr != nil {
            m.logger.Error("tx rollback failed", "error", rbErr)
        }
        return err
    }

    // コミット
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("db: commit: %w", err)
    }

    return nil
}

// runInSavepoint はネストしたトランザクションを Savepoint で実現する。
func (m *TxMgr) runInSavepoint(ctx context.Context, tx *sql.Tx, fn func(ctx context.Context) error) error {
    spName := fmt.Sprintf("broth_sp_%d", m.spID.Add(1))

    // SAVEPOINT の作成
    if _, err := tx.ExecContext(ctx, "SAVEPOINT "+spName); err != nil {
        return fmt.Errorf("db: savepoint %s: %w", spName, err)
    }

    // fn の実行
    if err := fn(ctx); err != nil {
        // ROLLBACK TO SAVEPOINT
        if _, rbErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+spName); rbErr != nil {
            m.logger.Error("savepoint rollback failed",
                "savepoint", spName, "error", rbErr)
        }
        return err
    }

    // RELEASE SAVEPOINT
    if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+spName); err != nil {
        return fmt.Errorf("db: release savepoint %s: %w", spName, err)
    }

    return nil
}
```

### 3.3 Service 層での使用例

```go
// modules/order/service.go
package order

import (
    "context"
    "fmt"

    "myapp/modules/account"
    "myapp/broth/db"
)

type Service struct {
    repo       Repository
    accountSvc *account.Service
    txMgr      db.TxManager
}

// PlaceOrder は注文を確定する。
// 複数のリポジトリ操作を1つのトランザクションで実行する。
func (s *Service) PlaceOrder(ctx context.Context, input PlaceOrderInput) (*Order, error) {
    if err := input.Validate(); err != nil {
        return nil, fmt.Errorf("order: validation: %w", err)
    }

    var order *Order

    // トランザクション内で複数の操作を実行
    err := s.txMgr.RunInTx(ctx, func(ctx context.Context) error {
        // 1. 在庫の確認と確保
        if err := s.repo.ReserveStock(ctx, input.Items); err != nil {
            return fmt.Errorf("reserve stock: %w", err)
        }

        // 2. 注文の作成
        var err error
        order, err = s.repo.CreateOrder(ctx, input)
        if err != nil {
            return fmt.Errorf("create order: %w", err)
        }

        // 3. 支払い記録の作成
        if err := s.repo.CreatePayment(ctx, order.ID, input.PaymentMethod); err != nil {
            return fmt.Errorf("create payment: %w", err)
        }

        return nil // コミット
    })
    if err != nil {
        return nil, fmt.Errorf("order: place: %w", err)
    }

    return order, nil
}
```

### 3.4 ネストしたトランザクション（Savepoint）の使用例

```go
// modules/order/service.go（続き）

// PlaceOrderWithNotification は注文確定 + 通知を行う。
// 通知の失敗が注文自体を巻き戻さない例。
func (s *Service) PlaceOrderWithNotification(ctx context.Context, input PlaceOrderInput) (*Order, error) {
    var order *Order

    err := s.txMgr.RunInTx(ctx, func(ctx context.Context) error {
        // 注文処理（外側のトランザクション）
        var err error
        order, err = s.createOrder(ctx, input)
        if err != nil {
            return err
        }

        // 通知ログの記録（Savepoint でネスト）
        // この中でエラーが起きても、外側のトランザクションは継続する
        notifErr := s.txMgr.RunInTx(ctx, func(ctx context.Context) error {
            return s.repo.CreateNotificationLog(ctx, order.ID, "order_placed")
        })
        if notifErr != nil {
            // 通知ログの失敗はログに記録するが、注文は成功させる
            s.logger.Warn("notification log failed",
                "order_id", order.ID, "error", notifErr)
        }

        return nil
    })
    if err != nil {
        return nil, fmt.Errorf("order: place with notification: %w", err)
    }

    return order, nil
}
```

### 3.5 トランザクション処理フロー図

```mermaid
sequenceDiagram
    participant S as Service
    participant TM as TxManager
    participant DB as database/sql
    participant R1 as Repository A
    participant R2 as Repository B

    S->>TM: RunInTx(ctx, fn)
    TM->>DB: BeginTx()
    DB-->>TM: *sql.Tx
    TM->>TM: ctx に *sql.Tx を格納

    Note over TM: fn(txCtx) を実行

    TM->>R1: Create(txCtx, ...)
    R1->>R1: ConnFromContext(txCtx) -> *sql.Tx
    R1->>DB: INSERT (via *sql.Tx)
    DB-->>R1: ok

    TM->>R2: Update(txCtx, ...)
    R2->>R2: ConnFromContext(txCtx) -> *sql.Tx
    R2->>DB: UPDATE (via *sql.Tx)
    DB-->>R2: ok

    alt fn が nil を返した場合
        TM->>DB: Commit()
        DB-->>TM: ok
    else fn が error を返した場合
        TM->>DB: Rollback()
        DB-->>TM: ok
    end

    TM-->>S: error (or nil)
```

---

## 4. マイグレーション設計

### 4.1 基本方針: SQL ファイルベースのマイグレーション

| 選択肢 | 評価 | 採用 |
|---|---|---|
| **goose** | Go 製。SQL ファイル対応。軽量。広く使われている | 採用 (内部ラップ) |
| Atlas | HCL ベースの宣言的マイグレーション。高機能だが学習コスト高 | 不採用 |
| golang-migrate | 機能的に十分だが、メンテナンスが不安定な時期がある | 不採用 |
| 自前実装 | フルコントロール可能だが、車輪の再発明 | 不採用 |

**判断根拠**:
- goose は SQL ファイルベースで Broth の「SQL を第一級市民にする」方針と合致する
- Go の `embed` パッケージとの親和性が高い
- 十分にシンプルで、ラップしても抽象化のコストが低い
- 自前実装は「バージョンの競合制御」「ロック機構」等の複雑さを引き受ける必要があり、コスト対効果が合わない
- `broth/migrate` パッケージで goose をラップし、CLI (`broth migrate`) として統合する

### 4.2 マイグレーションファイルの配置と命名

```
myapp/
├── db/
│   ├── migrations/                              # マイグレーションファイル
│   │   ├── 20260208120000_create_users.up.sql
│   │   ├── 20260208120000_create_users.down.sql
│   │   ├── 20260208130000_create_articles.up.sql
│   │   ├── 20260208130000_create_articles.down.sql
│   │   ├── 20260209100000_add_users_email_index.up.sql
│   │   └── 20260209100000_add_users_email_index.down.sql
│   └── seeds/                                   # シードデータ
│       ├── 001_admin_user.sql
│       └── 002_master_data.sql
└── ...
```

**命名規約**:
- `YYYYMMDDHHMMSS_description.up.sql` / `.down.sql`
- タイムスタンプはマイグレーション作成時刻（秒単位）
- description はスネークケースで簡潔に記述
- up.sql と down.sql は必ずペアで作成する

**配置ルール**:
- マイグレーションはモジュール単位ではなく、 `db/migrations/` に集約する
- 理由: DDL は複数モジュールのテーブルに跨ることがあり、実行順序の管理が単一ディレクトリの方が容易

### 4.3 マイグレーションファイルの例

```sql
-- db/migrations/20260208120000_create_users.up.sql

CREATE TABLE users (
    id            BIGSERIAL    PRIMARY KEY,
    email         VARCHAR(255) NOT NULL UNIQUE,
    name          VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);

CREATE INDEX idx_users_email ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_is_active ON users (is_active) WHERE deleted_at IS NULL;

COMMENT ON TABLE users IS 'ユーザーマスタ';
COMMENT ON COLUMN users.deleted_at IS '論理削除日時。NULLの場合は有効';
```

```sql
-- db/migrations/20260208120000_create_users.down.sql

DROP TABLE IF EXISTS users;
```

### 4.4 broth/migrate パッケージ

```go
// broth/migrate/migrate.go
package migrate

import (
    "context"
    "database/sql"
    "embed"
    "fmt"
    "log/slog"

    "github.com/pressly/goose/v3"
)

// Migrator はマイグレーションを管理する。
type Migrator struct {
    db     *sql.DB
    dir    string // マイグレーションファイルのディレクトリ
    logger *slog.Logger
}

// New は Migrator を生成する。
func New(db *sql.DB, dir string, logger *slog.Logger) *Migrator {
    return &Migrator{db: db, dir: dir, logger: logger}
}

// NewFromEmbed は embed.FS からマイグレーションを読み込む Migrator を生成する。
// 本番デプロイ時はバイナリ埋め込みを使用する。
func NewFromEmbed(db *sql.DB, fs embed.FS, dir string, logger *slog.Logger) *Migrator {
    goose.SetBaseFS(fs)
    return &Migrator{db: db, dir: dir, logger: logger}
}

// Up は全ての未適用マイグレーションを実行する。
func (m *Migrator) Up(ctx context.Context) error {
    m.logger.Info("running migrations up", "dir", m.dir)
    if err := goose.UpContext(ctx, m.db, m.dir); err != nil {
        return fmt.Errorf("migrate up: %w", err)
    }
    m.logger.Info("migrations completed")
    return nil
}

// UpTo は指定バージョンまでのマイグレーションを実行する。
func (m *Migrator) UpTo(ctx context.Context, version int64) error {
    m.logger.Info("running migrations up to", "version", version)
    if err := goose.UpToContext(ctx, m.db, m.dir, version); err != nil {
        return fmt.Errorf("migrate up to %d: %w", version, err)
    }
    return nil
}

// Down は直近1つのマイグレーションをロールバックする。
func (m *Migrator) Down(ctx context.Context) error {
    m.logger.Info("running migration down")
    if err := goose.DownContext(ctx, m.db, m.dir); err != nil {
        return fmt.Errorf("migrate down: %w", err)
    }
    return nil
}

// Status は全マイグレーションの適用状態を返す。
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
    // goose のステータス取得をラップ
    migrations, err := goose.CollectMigrations(m.dir, 0, goose.MaxVersion)
    if err != nil {
        return nil, fmt.Errorf("migrate status: %w", err)
    }

    current, err := goose.GetDBVersionContext(ctx, m.db)
    if err != nil {
        return nil, fmt.Errorf("migrate status: get version: %w", err)
    }

    var statuses []MigrationStatus
    for _, mg := range migrations {
        applied := mg.Version <= current
        statuses = append(statuses, MigrationStatus{
            Version: mg.Version,
            Name:    mg.Source,
            Applied: applied,
        })
    }
    return statuses, nil
}

// MigrationStatus はマイグレーションの適用状態。
type MigrationStatus struct {
    Version int64
    Name    string
    Applied bool
}
```

### 4.5 CLI コマンド

```
# 全ての未適用マイグレーションを実行
$ broth migrate up

# 直近1つをロールバック
$ broth migrate down

# 適用状態の確認
$ broth migrate status
    Applied  20260208120000_create_users
    Applied  20260208130000_create_articles
    Pending  20260209100000_add_users_email_index

# 新しいマイグレーションファイルを生成（タイムスタンプ自動付与）
$ broth migrate create add_orders_table
    Created db/migrations/20260209143022_add_orders_table.up.sql
    Created db/migrations/20260209143022_add_orders_table.down.sql

# 指定バージョンまで適用
$ broth migrate up --to 20260208130000
```

### 4.6 シーディング（初期データ投入）

```go
// broth/migrate/seed.go
package migrate

import (
    "context"
    "database/sql"
    "fmt"
    "log/slog"
    "os"
    "path/filepath"
    "sort"
)

// Seeder はシードデータの投入を管理する。
type Seeder struct {
    db     *sql.DB
    dir    string
    logger *slog.Logger
}

// NewSeeder は Seeder を生成する。
func NewSeeder(db *sql.DB, dir string, logger *slog.Logger) *Seeder {
    return &Seeder{db: db, dir: dir, logger: logger}
}

// Run は seeds/ 配下の全 SQL ファイルを番号順に実行する。
// 冪等であることをシードファイル側で保証する（INSERT ... ON CONFLICT DO NOTHING 等）。
func (s *Seeder) Run(ctx context.Context) error {
    files, err := filepath.Glob(filepath.Join(s.dir, "*.sql"))
    if err != nil {
        return fmt.Errorf("seed: glob: %w", err)
    }
    sort.Strings(files)

    for _, f := range files {
        s.logger.Info("running seed", "file", filepath.Base(f))
        content, err := os.ReadFile(f)
        if err != nil {
            return fmt.Errorf("seed: read %s: %w", f, err)
        }
        if _, err := s.db.ExecContext(ctx, string(content)); err != nil {
            return fmt.Errorf("seed: exec %s: %w", filepath.Base(f), err)
        }
    }

    s.logger.Info("seeding completed", "count", len(files))
    return nil
}
```

**シードファイルの例**:

```sql
-- db/seeds/001_admin_user.sql

-- 冪等に書く（ON CONFLICT DO NOTHING）
INSERT INTO users (email, name, password_hash, is_active, created_at, updated_at)
VALUES (
    'admin@example.com',
    'Admin',
    '$2a$10$XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX', -- bcrypt hash
    TRUE,
    NOW(),
    NOW()
)
ON CONFLICT (email) DO NOTHING;
```

**CLI**:

```
$ broth seed
    Running seed: 001_admin_user.sql
    Running seed: 002_master_data.sql
    Seeding completed (2 files)
```

### 4.7 本番デプロイ時のマイグレーション

```go
// cmd/server/main.go

import "embed"

//go:embed db/migrations/*.sql
var migrations embed.FS

func main() {
    // ...

    // バイナリに埋め込まれたマイグレーションを使用
    migrator := migrate.NewFromEmbed(database.DB(), migrations, "db/migrations", logger)

    // 起動時にマイグレーションを自動実行（オプション）
    if cfg.AutoMigrate {
        if err := migrator.Up(context.Background()); err != nil {
            slog.Error("migration failed", "error", err)
            os.Exit(1)
        }
    }
}
```

---

## 5. 管理画面（Admin）の構想

### 5.1 Django Admin の分析と Broth のアプローチ

Django Admin は「モデル定義 -> 実行時リフレクション -> 自動的に CRUD 画面を生成」するアーキテクチャを持つ。これを Go で実現するには、リフレクションの代わりにコード生成を使う。

```mermaid
graph LR
    subgraph "Django Admin（リフレクション）"
        DM["Model クラス"] -->|"実行時<br/>リフレクション"| DA["Admin 画面<br/>（動的生成）"]
    end

    subgraph "Broth Admin（コード生成）"
        OM["Go 構造体<br/>+ struct tag"] -->|"broth admin generate<br/>(コード生成)"| OA["Admin ハンドラ<br/>+ テンプレート<br/>（静的生成）"]
    end

    style DM fill:#50B848,color:#fff
    style DA fill:#50B848,color:#fff
    style OM fill:#4A90D9,color:#fff
    style OA fill:#4A90D9,color:#fff
```

**Django Admin vs Broth Admin**:

| 観点 | Django Admin | Broth Admin |
|---|---|---|
| 生成タイミング | 実行時（リフレクション） | ビルド前（コード生成） |
| カスタマイズ | ModelAdmin のオーバーライド | 生成コードの直接編集 + フック関数 |
| 型安全性 | Python の動的型付け | Go のコンパイル時チェック |
| 初回の手間 | `admin.site.register(Model)` 一行 | `broth admin generate <module>` コマンド実行 |
| 柔軟性 | 高い（Pythonの動的性） | 中程度（生成コードベース） |

### 5.2 Admin メタデータの定義（struct tag）

```go
// modules/account/model.go
package account

import "time"

// User はアカウントのドメインモデル。
// `broth` タグで Admin 画面の振る舞いを宣言する。
type User struct {
    ID           int64     `broth:"admin:list,search"            json:"id"`
    Email        string    `broth:"admin:list,search,edit"       json:"email"`
    Name         string    `broth:"admin:list,search,edit"       json:"name"`
    PasswordHash string    `broth:"-"                            json:"-"`
    IsActive     bool      `broth:"admin:list,edit,filter"       json:"is_active"`
    CreatedAt    time.Time `broth:"admin:list,readonly"          json:"created_at"`
    UpdatedAt    time.Time `broth:"admin:list,readonly"          json:"updated_at"`
}
```

**`broth` タグのルール**:

| タグ値 | 意味 |
|---|---|
| `admin:list` | 一覧画面に表示 |
| `admin:search` | 検索対象フィールド |
| `admin:edit` | 編集画面で編集可能 |
| `admin:filter` | フィルタリング対象 |
| `admin:readonly` | 一覧に表示するが編集不可 |
| `-` | Admin 画面に表示しない（パスワードハッシュ等） |

### 5.3 broth admin generate コマンド

```
$ broth admin generate account

Generated admin files for module 'account':
  modules/account/admin_handler.go      # Admin 画面のハンドラ
  modules/account/admin_routes.go       # Admin ルーティング
  modules/account/templates/admin/
    list.html                            # 一覧画面
    detail.html                          # 詳細画面
    form.html                            # 作成/編集フォーム
    delete_confirm.html                  # 削除確認
```

**生成フロー**:

```mermaid
sequenceDiagram
    participant Dev as 開発者
    participant CLI as broth admin generate
    participant AST as Go AST 解析
    participant Tag as struct tag パーサー
    participant Gen as テンプレート/コード生成器

    Dev->>CLI: broth admin generate account
    CLI->>AST: model.go を解析
    AST-->>CLI: User 構造体のフィールド情報
    CLI->>Tag: broth タグを解析
    Tag-->>CLI: Admin メタデータ (list/search/edit 等)
    CLI->>Gen: ハンドラ + テンプレート生成
    Gen-->>CLI: admin_handler.go, admin_routes.go, templates/admin/*.html
    CLI-->>Dev: 生成完了
```

### 5.4 生成される Admin ハンドラ

```go
// modules/account/admin_handler.go
// Code generated by broth admin generate. You may edit this file.

package account

import (
    "net/http"
    "strconv"

    "myapp/modules/shared"
    "myapp/broth/render"
)

// AdminHandler はアカウントモジュールの管理画面ハンドラ。
// 生成後にカスタマイズ可能。
type AdminHandler struct {
    svc      *Service
    renderer *render.Renderer
}

// NewAdminHandler は AdminHandler を生成する。
func NewAdminHandler(svc *Service, renderer *render.Renderer) *AdminHandler {
    return &AdminHandler{svc: svc, renderer: renderer}
}

// List はユーザー一覧を表示する。
func (h *AdminHandler) List(w http.ResponseWriter, r *http.Request) {
    page := shared.PageFromRequest(r) // クエリパラメータから Page を構築
    search := r.URL.Query().Get("q")

    filter := UserFilter{
        NameContains:  search,
        EmailContains: search,
    }

    result, err := h.svc.List(r.Context(), page, filter)
    if err != nil {
        h.renderer.Error(w, r, http.StatusInternalServerError, err)
        return
    }

    h.renderer.HTML(w, r, http.StatusOK, "account/admin/list.html", map[string]any{
        "Title":   "ユーザー管理",
        "Result":  result,
        "Search":  search,
        "Page":    page,
    })
}

// Detail はユーザー詳細を表示する。
func (h *AdminHandler) Detail(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
    if err != nil {
        h.renderer.Error(w, r, http.StatusBadRequest, err)
        return
    }

    user, err := h.svc.FindByID(r.Context(), id)
    if err != nil {
        h.renderer.Error(w, r, http.StatusInternalServerError, err)
        return
    }
    if user == nil {
        h.renderer.Error(w, r, http.StatusNotFound, nil)
        return
    }

    h.renderer.HTML(w, r, http.StatusOK, "account/admin/detail.html", map[string]any{
        "Title": "ユーザー詳細",
        "User":  user,
    })
}

// Create は新規ユーザー作成フォームを表示する。
func (h *AdminHandler) Create(w http.ResponseWriter, r *http.Request) {
    h.renderer.HTML(w, r, http.StatusOK, "account/admin/form.html", map[string]any{
        "Title":  "ユーザー作成",
        "IsNew":  true,
    })
}

// CreatePost は新規ユーザーを作成する。
func (h *AdminHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
    var input RegisterInput
    if err := parseForm(r, &input); err != nil {
        h.renderer.HTML(w, r, http.StatusUnprocessableEntity, "account/admin/form.html", map[string]any{
            "Title":  "ユーザー作成",
            "IsNew":  true,
            "Errors": err,
            "Input":  input,
        })
        return
    }

    _, err := h.svc.Register(r.Context(), input)
    if err != nil {
        h.renderer.HTML(w, r, http.StatusUnprocessableEntity, "account/admin/form.html", map[string]any{
            "Title":  "ユーザー作成",
            "IsNew":  true,
            "Errors": err,
            "Input":  input,
        })
        return
    }

    http.Redirect(w, r, "/admin/account/users", http.StatusSeeOther)
}

// DeleteConfirm は削除確認画面を表示する。
func (h *AdminHandler) DeleteConfirm(w http.ResponseWriter, r *http.Request) {
    id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
    user, err := h.svc.FindByID(r.Context(), id)
    if err != nil || user == nil {
        h.renderer.Error(w, r, http.StatusNotFound, err)
        return
    }

    h.renderer.HTML(w, r, http.StatusOK, "account/admin/delete_confirm.html", map[string]any{
        "Title": "削除確認",
        "User":  user,
    })
}

// DeletePost はユーザーを削除する。
func (h *AdminHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
    id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
    if err := h.svc.Delete(r.Context(), id); err != nil {
        h.renderer.Error(w, r, http.StatusInternalServerError, err)
        return
    }
    http.Redirect(w, r, "/admin/account/users", http.StatusSeeOther)
}
```

### 5.5 生成される Admin ルーティング

```go
// modules/account/admin_routes.go
// Code generated by broth admin generate. You may edit this file.

package account

import (
    "net/http"

    "myapp/broth/router"
)

// AdminRoutes は管理画面のルーティングを返す。
func (h *AdminHandler) AdminRoutes() []router.Route {
    return []router.Route{
        {Pattern: "GET /admin/account/users", Handler: http.HandlerFunc(h.List)},
        {Pattern: "GET /admin/account/users/new", Handler: http.HandlerFunc(h.Create)},
        {Pattern: "POST /admin/account/users/new", Handler: http.HandlerFunc(h.CreatePost)},
        {Pattern: "GET /admin/account/users/{id}", Handler: http.HandlerFunc(h.Detail)},
        {Pattern: "GET /admin/account/users/{id}/edit", Handler: http.HandlerFunc(h.Edit)},
        {Pattern: "POST /admin/account/users/{id}/edit", Handler: http.HandlerFunc(h.EditPost)},
        {Pattern: "GET /admin/account/users/{id}/delete", Handler: http.HandlerFunc(h.DeleteConfirm)},
        {Pattern: "POST /admin/account/users/{id}/delete", Handler: http.HandlerFunc(h.DeletePost)},
    }
}
```

### 5.6 Admin のカスタマイズフック

生成されたコードは直接編集可能だが、よくあるカスタマイズはフック関数で対応する。

```go
// modules/account/admin_hooks.go（開発者が手書き）
package account

import (
    "context"
    "net/http"
)

// AdminConfig は管理画面のカスタマイズ設定。
type AdminConfig struct {
    // BeforeCreate は作成前に実行されるフック。
    BeforeCreate func(ctx context.Context, input *RegisterInput) error

    // AfterDelete は削除後に実行されるフック。
    AfterDelete func(ctx context.Context, id int64) error

    // ListColumns は一覧画面に表示するカラムのオーバーライド。
    ListColumns []string

    // PageSize は一覧画面のデフォルトページサイズ。
    PageSize int

    // CanDelete は削除可能かを判定する関数。
    CanDelete func(r *http.Request, user *User) bool
}

// DefaultAdminConfig はデフォルトの Admin 設定。
func DefaultAdminConfig() AdminConfig {
    return AdminConfig{
        ListColumns: []string{"ID", "Email", "Name", "IsActive", "CreatedAt"},
        PageSize:    25,
        CanDelete: func(r *http.Request, user *User) bool {
            return true // デフォルトは全ユーザー削除可能
        },
    }
}
```

### 5.7 Admin 画面の段階的リリース計画

| Phase | スコープ | 内容 |
|---|---|---|
| **Phase 1** (v0.1) | CRUD 自動生成 | `broth admin generate` で一覧/詳細/作成/編集/削除の画面とハンドラを生成 |
| **Phase 2** (v0.2) | フィルタ + 検索 | struct tag の `filter` / `search` に基づくフィルタリングと全文検索 |
| **Phase 3** (v0.3) | ダッシュボード | Admin トップページに統計情報を表示。カスタムウィジェット対応 |
| **Phase 4** (v0.4) | 権限管理 | ロールベースの Admin アクセス制御。操作ログ（監査トレイル） |

---

## 6. テンプレートエンジン

### 6.1 html/template の拡張方針

Broth は Go 標準の `html/template` を基盤とし、Django テンプレートエンジンの利便性を最小限の抽象で実現する。

**拡張する点**:
- レイアウト継承（`base.html` -> 各ページが extend する仕組み）
- パーシャル / コンポーネント的な再利用
- 標準 FuncMap（日時フォーマット、URL 生成、CSRF トークン等）
- フォームバリデーションエラーの表示統合

**拡張しない点**:
- 独自テンプレート言語の発明
- リアクティブ UI（HTMX 等との連携はテンプレート内で直接記述）

### 6.2 テンプレートディレクトリ構造

```
myapp/
├── templates/                       # 共通テンプレート
│   ├── layouts/
│   │   ├── base.html                # 全ページ共通レイアウト
│   │   └── admin.html               # Admin 画面用レイアウト
│   ├── partials/
│   │   ├── header.html              # ヘッダー
│   │   ├── footer.html              # フッター
│   │   ├── pagination.html          # ページネーションUI
│   │   ├── flash.html               # フラッシュメッセージ
│   │   └── form_errors.html         # バリデーションエラー表示
│   └── errors/
│       ├── 404.html
│       └── 500.html
├── modules/
│   ├── account/
│   │   └── templates/               # モジュール固有テンプレート
│   │       ├── login.html
│   │       ├── register.html
│   │       └── admin/
│   │           ├── list.html
│   │           ├── detail.html
│   │           ├── form.html
│   │           └── delete_confirm.html
│   └── article/
│       └── templates/
│           ├── list.html
│           └── detail.html
```

### 6.3 レイアウト継承

```html
{{/* templates/layouts/base.html */}}
<!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{block "title" .}}Broth App{{end}}</title>
    {{block "head" .}}{{end}}
    <link rel="stylesheet" href="/static/css/main.css">
</head>
<body>
    {{template "partials/header.html" .}}

    {{template "partials/flash.html" .}}

    <main class="container">
        {{block "content" .}}{{end}}
    </main>

    {{template "partials/footer.html" .}}

    {{block "scripts" .}}{{end}}
</body>
</html>
```

```html
{{/* modules/account/templates/login.html */}}
{{extends "layouts/base.html"}}

{{define "title"}}ログイン - Broth App{{end}}

{{define "content"}}
<div class="auth-form">
    <h1>ログイン</h1>

    {{if .Errors}}
        {{template "partials/form_errors.html" .Errors}}
    {{end}}

    <form method="POST" action="/login">
        {{csrfField}}

        <div class="form-group">
            <label for="email">メールアドレス</label>
            <input type="email" id="email" name="email"
                   value="{{.Input.Email}}" required>
        </div>

        <div class="form-group">
            <label for="password">パスワード</label>
            <input type="password" id="password" name="password" required>
        </div>

        <button type="submit" class="btn btn-primary">ログイン</button>
    </form>
</div>
{{end}}
```

### 6.4 パーシャル / コンポーネント

```html
{{/* templates/partials/pagination.html */}}
{{if gt .TotalPages 1}}
<nav class="pagination" aria-label="ページネーション">
    {{if .HasPrev}}
        <a href="{{.PrevURL}}" class="pagination-prev">前へ</a>
    {{end}}

    {{range .PageNumbers}}
        {{if eq . $.CurrentPage}}
            <span class="pagination-current">{{.}}</span>
        {{else}}
            <a href="{{pageURL .}}" class="pagination-link">{{.}}</a>
        {{end}}
    {{end}}

    {{if .HasNext}}
        <a href="{{.NextURL}}" class="pagination-next">次へ</a>
    {{end}}
</nav>
{{end}}
```

```html
{{/* templates/partials/form_errors.html */}}
{{if .}}
<div class="alert alert-danger" role="alert">
    <ul class="error-list">
        {{range .Errors}}
            <li>{{.}}</li>
        {{end}}
    </ul>
</div>
{{end}}
```

### 6.5 broth/render パッケージ

```go
// broth/render/render.go
package render

import (
    "fmt"
    "html/template"
    "io"
    "io/fs"
    "net/http"
    "path/filepath"
    "sync"
)

// Renderer はテンプレートのレンダリングを管理する。
type Renderer struct {
    templates map[string]*template.Template
    funcMap   template.FuncMap
    dirs      []fs.FS   // テンプレートディレクトリ（共通 + モジュール別）
    layouts   fs.FS     // レイアウトディレクトリ
    partials  fs.FS     // パーシャルディレクトリ
    mu        sync.RWMutex
    devMode   bool      // true の場合、リクエスト毎にテンプレートを再読み込み
}

// Config は Renderer の設定。
type Config struct {
    TemplateDir string            // 共通テンプレートディレクトリ
    ModuleDirs  []string          // モジュール別テンプレートディレクトリ
    FuncMap     template.FuncMap  // カスタム関数
    DevMode     bool              // 開発モード（ホットリロード）
}

// New は Renderer を生成する。
func New(cfg Config) *Renderer {
    r := &Renderer{
        templates: make(map[string]*template.Template),
        funcMap:   defaultFuncMap(),
        devMode:   cfg.DevMode,
    }

    // カスタム FuncMap をマージ
    for k, v := range cfg.FuncMap {
        r.funcMap[k] = v
    }

    // テンプレートの初回読み込み
    r.loadTemplates(cfg)

    return r
}

// HTML はテンプレートをレンダリングしてレスポンスに書き込む。
func (r *Renderer) HTML(w http.ResponseWriter, req *http.Request, status int, name string, data map[string]any) {
    // 開発モードではリクエスト毎に再読み込み
    if r.devMode {
        r.reload()
    }

    r.mu.RLock()
    tmpl, ok := r.templates[name]
    r.mu.RUnlock()

    if !ok {
        http.Error(w, fmt.Sprintf("template %q not found", name), http.StatusInternalServerError)
        return
    }

    // 共通データの注入
    if data == nil {
        data = make(map[string]any)
    }
    data["CSRFToken"] = csrfTokenFromRequest(req)
    data["CurrentUser"] = currentUserFromRequest(req)
    data["Flash"] = flashFromRequest(req)

    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.WriteHeader(status)

    if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
        // テンプレートエラーは既にヘッダーを書き込んだ後なのでログのみ
        fmt.Fprintf(w, "<!-- template error: %v -->", err)
    }
}

// Error はエラーページをレンダリングする。
func (r *Renderer) Error(w http.ResponseWriter, req *http.Request, status int, err error) {
    name := fmt.Sprintf("errors/%d.html", status)
    r.HTML(w, req, status, name, map[string]any{
        "Error": err,
    })
}

// JSON は JSON レスポンスを返す（API エンドポイント用）。
func (r *Renderer) JSON(w http.ResponseWriter, status int, data any) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(status)
    if err := encodeJSON(w, data); err != nil {
        fmt.Fprintf(w, `{"error":"encode failed"}`)
    }
}
```

### 6.6 標準 FuncMap

```go
// broth/render/funcmap.go
package render

import (
    "fmt"
    "html/template"
    "strings"
    "time"
)

// defaultFuncMap は Broth が標準提供するテンプレート関数。
func defaultFuncMap() template.FuncMap {
    return template.FuncMap{
        // --- 日時フォーマット ---
        "formatDate": func(t time.Time) string {
            return t.Format("2006-01-02")
        },
        "formatDateTime": func(t time.Time) string {
            return t.Format("2006-01-02 15:04")
        },
        "timeAgo": func(t time.Time) string {
            d := time.Since(t)
            switch {
            case d < time.Minute:
                return "たった今"
            case d < time.Hour:
                return fmt.Sprintf("%d分前", int(d.Minutes()))
            case d < 24*time.Hour:
                return fmt.Sprintf("%d時間前", int(d.Hours()))
            default:
                return fmt.Sprintf("%d日前", int(d.Hours()/24))
            }
        },

        // --- URL生成 ---
        "urlFor": func(name string, params ...any) string {
            // ルート名から URL を生成する（broth/router と連携）
            // 実装は Router のルックアップテーブルを参照する
            return fmt.Sprintf("/%s", name) // 簡略版
        },

        // --- CSRF ---
        "csrfField": func() template.HTML {
            // 実際にはリクエストコンテキストから取得する
            return template.HTML(`<input type="hidden" name="_csrf" value="">`)
        },

        // --- 文字列ヘルパー ---
        "truncate": func(s string, n int) string {
            if len(s) <= n {
                return s
            }
            return s[:n] + "..."
        },
        "upper": strings.ToUpper,
        "lower": strings.ToLower,

        // --- 条件ヘルパー ---
        "eq":  func(a, b any) bool { return fmt.Sprint(a) == fmt.Sprint(b) },
        "neq": func(a, b any) bool { return fmt.Sprint(a) != fmt.Sprint(b) },

        // --- HTML ヘルパー ---
        "safeHTML": func(s string) template.HTML {
            return template.HTML(s)
        },

        // --- 数値ヘルパー ---
        "add": func(a, b int) int { return a + b },
        "sub": func(a, b int) int { return a - b },

        // --- コレクションヘルパー ---
        "seq": func(start, end int) []int {
            var s []int
            for i := start; i <= end; i++ {
                s = append(s, i)
            }
            return s
        },

        // --- レイアウト ---
        "extends": func(name string) string {
            // テンプレート継承のマーカー（ロード時に解釈される）
            return ""
        },
    }
}
```

### 6.7 broth/form パッケージ

フォームバインディング、バリデーション、エラー表示を統合するパッケージ。

```go
// broth/form/form.go
package form

import (
    "fmt"
    "net/http"
    "reflect"
    "strconv"
    "strings"
)

// Errors はフィールド単位のバリデーションエラーを保持する。
type Errors map[string][]string

// HasErrors はエラーが存在するかを返す。
func (e Errors) HasErrors() bool { return len(e) > 0 }

// Add はフィールドにエラーメッセージを追加する。
func (e Errors) Add(field, msg string) {
    e[field] = append(e[field], msg)
}

// Get はフィールドのエラーメッセージを返す。
func (e Errors) Get(field string) []string {
    return e[field]
}

// First はフィールドの最初のエラーメッセージを返す。
func (e Errors) First(field string) string {
    if msgs := e[field]; len(msgs) > 0 {
        return msgs[0]
    }
    return ""
}

// Bind は HTTP リクエストから構造体にフォーム値をバインドする。
// struct tag `form:"field_name"` でマッピングする。
func Bind(r *http.Request, dst any) error {
    if err := r.ParseForm(); err != nil {
        return fmt.Errorf("form: parse: %w", err)
    }

    v := reflect.ValueOf(dst)
    if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
        return fmt.Errorf("form: dst must be a pointer to struct")
    }
    v = v.Elem()
    t := v.Type()

    for i := 0; i < t.NumField(); i++ {
        field := t.Field(i)
        tag := field.Tag.Get("form")
        if tag == "" || tag == "-" {
            continue
        }

        formValue := r.FormValue(tag)
        if err := setField(v.Field(i), formValue); err != nil {
            return fmt.Errorf("form: field %s: %w", tag, err)
        }
    }

    return nil
}

// setField は文字列値を適切な型に変換してフィールドに設定する。
func setField(field reflect.Value, value string) error {
    switch field.Kind() {
    case reflect.String:
        field.SetString(value)
    case reflect.Int, reflect.Int64:
        if value == "" {
            return nil
        }
        n, err := strconv.ParseInt(value, 10, 64)
        if err != nil {
            return err
        }
        field.SetInt(n)
    case reflect.Bool:
        field.SetBool(value == "on" || value == "true" || value == "1")
    default:
        return fmt.Errorf("unsupported type: %s", field.Kind())
    }
    return nil
}

// Validate はバリデーションタグに基づいてフォーム値を検証する。
// struct tag `validate:"required,min=8,max=255"` でルールを定義する。
func Validate(src any) Errors {
    errs := make(Errors)

    v := reflect.ValueOf(src)
    if v.Kind() == reflect.Ptr {
        v = v.Elem()
    }
    t := v.Type()

    for i := 0; i < t.NumField(); i++ {
        field := t.Field(i)
        tag := field.Tag.Get("validate")
        if tag == "" {
            continue
        }

        formTag := field.Tag.Get("form")
        if formTag == "" {
            formTag = strings.ToLower(field.Name)
        }

        label := field.Tag.Get("label")
        if label == "" {
            label = field.Name
        }

        value := v.Field(i)
        rules := strings.Split(tag, ",")

        for _, rule := range rules {
            if err := applyRule(rule, label, value); err != nil {
                errs.Add(formTag, err.Error())
            }
        }
    }

    return errs
}

// applyRule は個別のバリデーションルールを適用する。
func applyRule(rule string, label string, value reflect.Value) error {
    parts := strings.SplitN(rule, "=", 2)
    ruleName := parts[0]

    switch ruleName {
    case "required":
        if value.IsZero() {
            return fmt.Errorf("%sは必須です", label)
        }
    case "min":
        if len(parts) < 2 {
            return nil
        }
        min, _ := strconv.Atoi(parts[1])
        if value.Kind() == reflect.String && len(value.String()) < min {
            return fmt.Errorf("%sは%d文字以上で入力してください", label, min)
        }
    case "max":
        if len(parts) < 2 {
            return nil
        }
        max, _ := strconv.Atoi(parts[1])
        if value.Kind() == reflect.String && len(value.String()) > max {
            return fmt.Errorf("%sは%d文字以下で入力してください", label, max)
        }
    case "email":
        if value.Kind() == reflect.String && !strings.Contains(value.String(), "@") {
            return fmt.Errorf("%sは有効なメールアドレスを入力してください", label)
        }
    }
    return nil
}
```

### 6.8 フォーム定義の例

```go
// modules/account/forms.go
package account

import (
    "net/http"

    "myapp/broth/form"
)

// RegisterForm はユーザー登録フォーム。
type RegisterForm struct {
    Email    string `form:"email"    validate:"required,email"    label:"メールアドレス"`
    Name     string `form:"name"     validate:"required,max=255"  label:"名前"`
    Password string `form:"password" validate:"required,min=8"    label:"パスワード"`
}

// BindAndValidate はリクエストからバインドし、バリデーションを実行する。
func (f *RegisterForm) BindAndValidate(r *http.Request) form.Errors {
    if err := form.Bind(r, f); err != nil {
        errs := make(form.Errors)
        errs.Add("_form", err.Error())
        return errs
    }
    return form.Validate(f)
}

// ToInput はフォーム値をドメインの入力型に変換する。
func (f *RegisterForm) ToInput() RegisterInput {
    return RegisterInput{
        Email:    f.Email,
        Name:     f.Name,
        Password: f.Password,
    }
}
```

### 6.9 ハンドラでのフォーム統合

```go
// modules/account/handler.go
package account

import (
    "net/http"

    "myapp/broth/render"
)

// Register はユーザー登録を処理する。
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodGet {
        // 空のフォームを表示
        h.renderer.HTML(w, r, http.StatusOK, "account/register.html", map[string]any{
            "Title": "ユーザー登録",
            "Form":  &RegisterForm{},
        })
        return
    }

    // POST: フォームのバインドとバリデーション
    f := &RegisterForm{}
    errs := f.BindAndValidate(r)
    if errs.HasErrors() {
        // バリデーションエラー: フォームを再表示（入力値を保持）
        h.renderer.HTML(w, r, http.StatusUnprocessableEntity, "account/register.html", map[string]any{
            "Title":  "ユーザー登録",
            "Form":   f,
            "Errors": errs,
        })
        return
    }

    // ビジネスロジックの実行
    _, err := h.svc.Register(r.Context(), f.ToInput())
    if err != nil {
        h.renderer.HTML(w, r, http.StatusUnprocessableEntity, "account/register.html", map[string]any{
            "Title":  "ユーザー登録",
            "Form":   f,
            "Errors": map[string][]string{"_form": {err.Error()}},
        })
        return
    }

    http.Redirect(w, r, "/login?registered=true", http.StatusSeeOther)
}
```

### 6.10 テンプレートとバリデーションの統合

```html
{{/* modules/account/templates/register.html */}}
{{extends "layouts/base.html"}}

{{define "title"}}ユーザー登録 - Broth App{{end}}

{{define "content"}}
<div class="auth-form">
    <h1>ユーザー登録</h1>

    {{if .Errors}}
        {{template "partials/form_errors.html" .Errors}}
    {{end}}

    <form method="POST" action="/register">
        {{csrfField}}

        <div class="form-group {{if .Errors.Get "email"}}has-error{{end}}">
            <label for="email">メールアドレス</label>
            <input type="email" id="email" name="email"
                   value="{{.Form.Email}}"
                   class="form-control" required>
            {{with .Errors.First "email"}}
                <span class="help-block text-danger">{{.}}</span>
            {{end}}
        </div>

        <div class="form-group {{if .Errors.Get "name"}}has-error{{end}}">
            <label for="name">名前</label>
            <input type="text" id="name" name="name"
                   value="{{.Form.Name}}"
                   class="form-control" required>
            {{with .Errors.First "name"}}
                <span class="help-block text-danger">{{.}}</span>
            {{end}}
        </div>

        <div class="form-group {{if .Errors.Get "password"}}has-error{{end}}">
            <label for="password">パスワード</label>
            <input type="password" id="password" name="password"
                   class="form-control" required>
            {{with .Errors.First "password"}}
                <span class="help-block text-danger">{{.}}</span>
            {{end}}
            <small class="form-text text-muted">8文字以上で入力してください</small>
        </div>

        <button type="submit" class="btn btn-primary">登録</button>
    </form>

    <p class="mt-3">既にアカウントをお持ちですか？ <a href="/login">ログイン</a></p>
</div>
{{end}}
```

### 6.11 テンプレートエンジンのアーキテクチャ図

```mermaid
graph TB
    subgraph "broth/render"
        RENDERER["Renderer<br/>テンプレート管理"]
        FUNCMAP["FuncMap<br/>標準テンプレート関数"]
        LOADER["TemplateLoader<br/>レイアウト継承解決"]
    end

    subgraph "broth/form"
        BIND["Bind<br/>HTTP -> struct"]
        VALIDATE["Validate<br/>struct tag バリデーション"]
        ERRORS["Errors<br/>フィールド単位エラー"]
    end

    subgraph "テンプレートファイル"
        LAYOUT["layouts/base.html"]
        PARTIAL["partials/*.html"]
        PAGE["modules/*/templates/*.html"]
    end

    subgraph "Handler (HTTP レイヤー)"
        HANDLER["handler.go"]
    end

    HANDLER --> RENDERER
    HANDLER --> BIND
    HANDLER --> VALIDATE
    BIND --> ERRORS
    VALIDATE --> ERRORS
    RENDERER --> FUNCMAP
    RENDERER --> LOADER
    LOADER --> LAYOUT
    LOADER --> PARTIAL
    LOADER --> PAGE
    RENDERER --> ERRORS

    style RENDERER fill:#4A90D9,color:#fff
    style FUNCMAP fill:#4A90D9,color:#fff
    style LOADER fill:#4A90D9,color:#fff
    style BIND fill:#E8724A,color:#fff
    style VALIDATE fill:#E8724A,color:#fff
    style ERRORS fill:#E8724A,color:#fff
    style LAYOUT fill:#F5A623,color:#fff
    style PARTIAL fill:#F5A623,color:#fff
    style PAGE fill:#F5A623,color:#fff
    style HANDLER fill:#50B848,color:#fff
```

---

## 7. 機能比較表

### データアクセス機能比較

| 機能 | Django ORM | Rails AR | GORM | ent | sqlc | **Broth (Bob)** |
|---|---|---|---|---|---|---|
| **アプローチ** | Active Record | Active Record | Active Record | コード生成 | SQL -> Go | **DB-first コード生成 (Bob)** |
| **型安全性** | 低 (Python) | 低 (Ruby) | 中 (リフレクション) | 高 (コード生成) | 高 (コード生成) | **高 (コード生成)** |
| **SQL の可視性** | 低 (ORM DSL) | 低 (ORM DSL) | 中 (チェーン) | 中 (DSL) | 高 (生SQL) | **高 (型安全 Query Mod + 生SQL)** |
| **マイグレーション** | 内蔵 (自動生成) | 内蔵 (自動生成) | AutoMigrate | 内蔵 (自動生成) | なし | **内蔵 (SQL ファイル / goose)** |
| **リレーション** | 宣言的 | 宣言的 | タグ定義 | エッジ定義 | なし | **Eager Loading (LEFT JOIN)** |
| **N+1 問題対策** | select_related | includes/eager | Preload | Eager Loading | なし (手書き) | **Eager Loading (Bob Preload)** |
| **トランザクション** | デコレータ | ブロック | Session | Client.Tx | なし | **RunInTx + ctx 伝搬** |
| **クエリビルダ** | 内蔵 (強力) | 内蔵 (強力) | 内蔵 (チェーン) | 内蔵 (DSL) | なし | **型安全 Query Mod (Bob)** |
| **Admin 画面** | 内蔵 (自動生成) | gem (ActiveAdmin) | なし | なし | なし | **内蔵 (コード生成)** |
| **学習コスト** | 中 | 中 | 中 | 高 | 低 | **低 (SQL + Go + Bob)** |
| **リフレクション** | N/A (Python) | N/A (Ruby) | 多い | なし | なし | **なし (コード生成)** |
| **テストファクトリー** | Factory Boy | FactoryBot | なし | なし | なし | **内蔵 (Bob factory_bot 風)** |
| **サードパーティ依存** | N/A | N/A | なし | なし | なし | **goose, Bob** |

### Broth の差別化ポイント

```mermaid
graph LR
    subgraph "Broth の独自価値"
        A["DB-first コード生成<br/>(Bob ベース・型安全)"]
        B["Admin 画面<br/>(Django ライク)"]
        C["FW 統合<br/>(全て broth コマンドで完結)"]
        D["レイヤー規約<br/>(Repository パターン)"]
    end

    subgraph "他FWとの比較"
        E["sqlc: SQL -> Go は得意だが<br/>リレーション操作・FW統合なし"]
        F["ent: コード生成は得意だが<br/>独自 DSL の学習コスト"]
        G["GORM: 手軽だが<br/>リフレクション・interface{}に課題"]
        H["Django: Admin は最強だが<br/>Go ではない"]
    end

    A --- E
    B --- H
    C --- F
    D --- G

    style A fill:#4A90D9,color:#fff
    style B fill:#E8724A,color:#fff
    style C fill:#50B848,color:#fff
    style D fill:#9B59B6,color:#fff
```

### 段階的な成熟度ロードマップ

| Phase | バージョン | データアクセス | Admin | テンプレート |
|---|---|---|---|---|
| **Phase 1** | v0.1 | Bob + broth/db パッケージ（CRUD・Eager Loading・ファクトリー） | CRUD コード生成 | レイアウト + パーシャル |
| **Phase 2** | v0.2 | broth generate model の拡張（bobgen-sql 検証、カスタムクエリ対応） | フィルタ + 検索 | FuncMap 拡張 |
| **Phase 3** | v0.3 | パフォーマンス計装 (スロークエリ検出・クエリログ) | ダッシュボード | フォーム統合 |
| **Phase 4** | v0.4 | Bob コントリビュート成果の統合・v1 追従 | 権限管理 + 監査ログ | ホットリロード |

---

## 付録: 設計判断の記録

### ADR-D001: Bob（database-first コード生成）を推奨データアクセスライブラリとして採用

**状況**: Go にはGORM, ent, Bun, sqlc, SQLBoiler, Bob 等のデータアクセスライブラリがある。

**決定**: Bob（`github.com/stephenafamo/bob`）を推奨データアクセスライブラリとして採用する。`broth/db` は Bob を薄くラップし、`broth generate model` は内部で `bobgen-psql` を呼び出す。

**根拠**:
- Bob は SQLBoiler のメンテナーが設計し直した後継であり、database-first・コード生成・型安全・`interface{}` 不使用を全て満たす
- GORM はリフレクション多用・`interface{}` 氾濫が Broth の設計原則と根本的に衝突（ADR-D007 参照）
- sqlc は設計原則との整合は高いが、JOIN → フラット構造体のマッピングコストが実務で重く、リレーション操作の生産性が不足
- SQLBoiler は思想的に近いが、メンテナンス体制が不安定で後継の Bob が存在する
- Bob のテスト用ファクトリー生成（factory_bot インスパイア）は Broth のテスト戦略に直接貢献
- `bob.Executor` インターフェースが `*sql.DB` / `*sql.Tx` を受け入れるため、`broth/db` の `ConnFromContext` との統合が自然

**リスク対策**: Bob は v1 未到達（v0.30 台）だが、Broth 自体も pre-v1 のため API 変更は許容可能。Phase 1 完了後にメンテ状況を再評価し、問題があればフォーク（D3）に切り替え可能。

**変更履歴**: 初期設計では「ORM 不採用、database/sql + コード生成（sqlc 推奨）」としていたが、リレーション操作の生産性と設計原則との整合を総合評価し、Bob 採用に変更

### ADR-D002: マイグレーションに goose を採用

**状況**: マイグレーションツールの選定。

**決定**: goose を broth/migrate パッケージで内部的にラップして使用する。

**根拠**:
- SQL ファイルベースのマイグレーションを自然にサポートする
- Go の embed.FS との統合が容易（本番バイナリに埋め込み可能）
- 自前実装はバージョン管理・ロック・競合制御の複雑さを引き受ける必要があり、ROI が合わない
- goose の API は十分にシンプルで、ラッピングのオーバーヘッドが小さい
- Atlas は高機能だが HCL ベースの宣言的アプローチが Broth の「SQL 第一級市民」方針と合わない

### ADR-D003: Admin 画面はコード生成ベース

**状況**: Django Admin 相当の機能を Go で実現する方法。

**決定**: `broth admin generate` コマンドで Go ハンドラ + HTML テンプレートを生成する。

**根拠**:
- Django Admin のリフレクションベースの動的生成は、Go のコンパイル時安全性の哲学に反する
- コード生成であれば、生成後に開発者が自由にカスタマイズできる
- 生成コードは Go のコンパイラが型チェックするため、安全性が保証される
- 初期コストは Django Admin より高いが、カスタマイズ時の透明性で勝る

### ADR-D004: テンプレートエンジンは html/template 拡張

**状況**: テンプレートエンジンの選定。

**決定**: Go 標準の html/template を拡張する。外部テンプレートエンジン（plush, jet 等）は採用しない。

**根拠**:
- html/template は自動エスケープ付きで XSS 対策が標準
- サードパーティ依存を最小限にする方針に合致
- レイアウト継承は `template.ParseFiles` の組み合わせと命名規約で実現可能
- FuncMap による拡張で十分な表現力を確保できる

### ADR-D005: broth/form でのリフレクション使用

**状況**: フォームバインディングでリフレクションを使うか。

**決定**: broth/form でのリフレクション使用を許可する（ARCHITECTURE.md ADR-003 で明示的に許可された範囲）。

**根拠**:
- フォームバインディングは `encoding/json` と同じパターンであり、Go コミュニティで広く受け入れられている
- コード生成で代替すると、フォーム定義のたびに `go generate` が必要になり開発体験が低下する
- リフレクションの使用範囲を `Bind()` と `Validate()` の2関数に限定し、影響範囲を最小化する

### ADR-D006: `broth generate model` は bobgen-psql をラップ

**状況**: データアクセスのコード生成をどのツールで行うか。

**決定**: `broth generate model` は内部で `bobgen-psql` を呼び出し、DB スキーマからモデルコード・テスト用ファクトリーを生成する。複雑な手書き SQL クエリには `bobgen-sql` を優先的に使用し、不足する場合は sqlc を補助的に利用可能とする。

**根拠**:
- Bob は DB スキーマから直接コード生成するため、SQL ファイルを手書きする必要がない（CRUD の大幅な省力化）
- ADR-D002（goose ラップ）と同じ「既存ツールのラップ/連携」路線で一貫性がある
- テスト用ファクトリーの自動生成が Broth のテスト戦略（PROJECT_STRUCTURE.md sec 8）と直接統合
- sqlc は Bob で表現しにくい複雑なクエリの補助として位置づけ、排他的な選択ではなく併用可能

**影響**: Bob がプロジェクトの依存に加わる。サードパーティ依存は最小限の原則に対し、「セキュリティライブラリと同様に、設計原則に合致する実績あるライブラリは例外として許容」の方針を適用する。

**変更履歴**: 初期設計では sqlc ラッパーとしていたが、Bob 採用決定（ADR-D001 改訂）に伴い変更

### ADR-D007: GORM を採用しない

**状況**: GORM は Go で最も利用されている ORM（GitHub 37k+ stars, 83,500+ importers）であり、採用を検討する価値がある。

**決定**: GORM は採用しない。技術的に利用可能（`broth/db.DB()` が `*sql.DB` を返すため）だが、Broth の公式推奨にはしない。

**根拠**:
- **`interface{}` の氾濫**: `Create`, `Find`, `Where`, `Update`, `Delete` 等の全公開メソッドが `interface{}` を受け入れ、Broth の「`interface{}` を公開 API に使わない」原則と正面衝突する
- **リフレクション多用**: 構造体タグの解析、フィールドマッピング、型変換が全て実行時リフレクションで、Broth の「リフレクションは最小限」原則に反する
- **暗黙の振る舞い**: `gorm.Model` 埋め込みによるソフトデリートの自動有効化、AutoMigrate のカラム残留等、Go の「明示性」文化と AI 生成コードの予測可能性を損なう
- **パフォーマンス**: ベンチマークで `database/sql` の 5-6 倍遅い（リフレクションのオーバーヘッド）

**補足**: GORM は「最も簡潔な CRUD 体験」を提供するが、その利便性はリフレクションと `interface{}` に依存しており、Broth が守る設計原則とトレードオフの関係にある。Bob はコード生成により GORM に近い生産性を、型安全に提供する。
