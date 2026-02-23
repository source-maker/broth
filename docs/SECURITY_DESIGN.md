# Broth -- セキュリティ設計書

> **バージョン**: 0.1.0-draft
> **最終更新**: 2026-02-08
> **ステータス**: 初期設計
> **前提ドキュメント**: [ARCHITECTURE.md](./ARCHITECTURE.md), [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md), [MODULE_DESIGN.md](./MODULE_DESIGN.md)

---

## 目次

1. [設計思想](#1-設計思想)
2. [セキュリティアーキテクチャ全体像](#2-セキュリティアーキテクチャ全体像)
3. [コンテキスト自動適用（差別化機能）](#3-コンテキスト自動適用差別化機能)
4. [認証（AuthN）設計](#4-認証authn設計)
5. [認可（AuthZ）設計](#5-認可authz設計)
6. [基本防御](#6-基本防御)
7. [秘密情報管理](#7-秘密情報管理)
8. [セキュリティヘッダのデフォルト設定](#8-セキュリティヘッダのデフォルト設定)
9. [設計判断の記録](#9-設計判断の記録)

---

## 1. 設計思想

### Secure by Default

Broth のセキュリティ設計は **「デフォルトで安全、明示的にオプトアウト」** を最上位原則とする。Django/Laravel が確立したこの思想を、Go のイディオム（明示性・型安全・コンパイル時チェック）で実現する。

| 原則 | 説明 | 具体例 |
|---|---|---|
| **デフォルトで安全** | `broth new` で生成されたプロジェクトは、セキュリティ設定なしで安全に動作する | CSRF保護・セキュリティヘッダが最初から有効 |
| **明示的なオプトアウト** | セキュリティ機能を無効にするには、明示的なコードが必要 | `middleware.CSRF(csrf.WithDisabled(true))` |
| **文脈に応じた自動適用** | SSR/API を自動判定し、適切なセキュリティスタックを選択する | SSRならCSRF ON、APIならCSRF OFF |
| **設定ミスの早期検出** | 起動時に必須設定（SECRET_KEY等）の検証を行い、不備があれば起動を拒否する | SECRET_KEY 未設定で起動するとパニック |
| **型安全な API** | セキュリティ関連の設定・操作に `interface{}` を使わない | `auth.User` 型でユーザー情報にアクセス |

### 既存フレームワークからの教訓

| フレームワーク | 学ぶべき点 | Broth での適用 |
|---|---|---|
| **Django** | CSRFミドルウェア + テンプレートタグの一体設計 | `middleware.CSRF()` + `{{csrfField}}` テンプレート関数 |
| **Django** | `settings.SECRET_KEY` の必須化 | `config.Security.SecretKey` 未設定で起動拒否 |
| **Laravel** | レート制限の標準搭載 | `middleware.RateLimit()` をコアに搭載 |
| **ASP.NET** | 認証スキームの切り替え機構 | コンテキスト自動適用で SSR/API を自動切替 |
| **Rails** | `protect_from_forgery` のデフォルト有効化 | CSRF保護をグローバルミドルウェアでデフォルト有効 |

---

## 2. セキュリティアーキテクチャ全体像

### セキュリティモジュール構成

ARCHITECTURE.md sec 4 のコアモジュール構成と整合する。

```mermaid
graph TB
    subgraph "broth/middleware（HTTPレイヤー）"
        MW_SEC["SecurityHeaders"]
        MW_CSRF["CSRF"]
        MW_CORS["CORS"]
        MW_AUTH["Auth"]
        MW_RATE["RateLimit"]
        MW_CTX["ContextDetect<br/>（SSR/API自動判定）"]
    end

    subgraph "broth/auth（横断的関心事）"
        AUTH_SESSION["SessionAuth<br/>セッション認証"]
        AUTH_BEARER["BearerAuth<br/>トークン認証"]
        AUTH_HASH["PasswordHash<br/>パスワードハッシュ"]
        AUTH_RBAC["RBAC<br/>ロールベース認可"]
        AUTH_POLICY["Policy<br/>リソースベース認可"]
        AUTH_OAUTH["OAuth2<br/>外部プロバイダ連携"]
    end

    subgraph "broth/session（HTTPレイヤー基盤）"
        SESS_COOKIE["CookieStore"]
        SESS_DB["DBStore"]
        SESS_REDIS["RedisStore"]
    end

    MW_CTX --> MW_AUTH
    MW_CTX --> MW_CSRF
    MW_AUTH --> AUTH_SESSION
    MW_AUTH --> AUTH_BEARER
    AUTH_SESSION --> SESS_COOKIE
    AUTH_SESSION --> SESS_DB
    AUTH_SESSION --> SESS_REDIS
    AUTH_RBAC --> AUTH_SESSION
    AUTH_RBAC --> AUTH_BEARER

    style MW_CTX fill:#E74C3C,color:#fff
    style MW_CSRF fill:#E8724A,color:#fff
    style MW_AUTH fill:#E8724A,color:#fff
    style MW_RATE fill:#E8724A,color:#fff
    style MW_CORS fill:#E8724A,color:#fff
    style MW_SEC fill:#E8724A,color:#fff
    style AUTH_SESSION fill:#4A90D9,color:#fff
    style AUTH_BEARER fill:#4A90D9,color:#fff
    style AUTH_HASH fill:#4A90D9,color:#fff
    style AUTH_RBAC fill:#9B59B6,color:#fff
    style AUTH_POLICY fill:#9B59B6,color:#fff
    style AUTH_OAUTH fill:#4A90D9,color:#fff
    style SESS_COOKIE fill:#50B848,color:#fff
    style SESS_DB fill:#50B848,color:#fff
    style SESS_REDIS fill:#50B848,color:#fff
```

### ミドルウェアチェーンとセキュリティ

ARCHITECTURE.md sec 3.1 で定義されたミドルウェアチェーンにセキュリティを統合する。PROJECT_STRUCTURE.md の `config/middleware.go` で設定する。

```
リクエスト
  │
  ▼
Recovery          ← パニックからの回復
  │
  ▼
RequestID         ← リクエスト追跡
  │
  ▼
SecurityHeaders   ← セキュリティヘッダの自動付与（★ 新規）
  │
  ▼
Logging           ← アクセスログ記録
  │
  ▼
Tracing           ← 分散トレーシング
  │
  ▼
ContextDetect     ← SSR/API コンテキスト判定（★ 差別化機能）
  │
  ▼
RateLimit         ← レート制限（★ 新規）
  │
  ▼
Session           ← セッション復元
  │
  ▼
Auth              ← 認証（セッション or Bearer を自動選択）
  │
  ▼
CSRF              ← CSRF保護（SSRのみ自動適用）
  │
  ▼
CORS              ← CORS設定（APIのみ自動適用）
  │
  ▼
Handler           ← ビジネスロジックの実行
```

### config/middleware.go への統合

PROJECT_STRUCTURE.md sec 3.2 で定義された `config/middleware.go` にセキュリティミドルウェアを追加する。

```go
// config/middleware.go
package config

import (
    "github.com/source-maker/broth/log"
    "github.com/source-maker/broth/middleware"
)

// GlobalMiddleware はアプリケーション全体に適用するミドルウェアを返す。
// 適用順序: 外側（先頭）から内側（末尾）へ。
//
//   Recovery → RequestID → SecurityHeaders → Logging → Tracing
//   → ContextDetect → RateLimit → Session → Auth → CSRF → CORS
//
func GlobalMiddleware(cfg *Config, logger *log.Logger) []middleware.Middleware {
    return []middleware.Middleware{
        middleware.Recovery(logger),
        middleware.RequestID(),
        middleware.SecurityHeaders(),                           // デフォルトのセキュリティヘッダ
        middleware.Logger(logger),
        middleware.Tracing(),
        middleware.ContextDetect(),                             // ★ SSR/API 自動判定
        middleware.RateLimit(cfg.Security.RateLimit),           // グローバルレート制限
        middleware.Session(cfg.Session),                        // セッション管理
        middleware.Auth(cfg.Security.Auth),                     // 認証（文脈で自動切替）
        middleware.CSRF(cfg.Security.CSRF),                     // CSRF（SSRのみ自動適用）
        middleware.CORS(cfg.Security.CORS),                     // CORS（APIのみ自動適用）
    }
}
```

### config/security.go -- セキュリティ設定構造体

PROJECT_STRUCTURE.md sec 3.2 のconfig構成に準拠し、セキュリティ専用の設定ファイルを追加する。

```go
// config/security.go
package config

import "time"

// SecurityConfig はセキュリティ関連の設定をまとめる。
type SecurityConfig struct {
    // SecretKey はセッション署名・CSRFトークン生成等に使う秘密鍵。
    // 必須。未設定の場合は起動時にパニックする。
    SecretKey string `env:"SECRET_KEY" required:"true"`

    Auth      AuthConfig
    CSRF      CSRFConfig
    CORS      CORSConfig
    RateLimit RateLimitConfig
}

// AuthConfig は認証設定。
type AuthConfig struct {
    // SessionLifetime はセッションの有効期間。
    SessionLifetime time.Duration `env:"AUTH_SESSION_LIFETIME" default:"24h"`
    // RememberMeDuration は Remember Me の有効期間。
    RememberMeDuration time.Duration `env:"AUTH_REMEMBER_ME_DURATION" default:"720h"` // 30日
    // BearerTokenLifetime はアクセストークンの有効期間。
    BearerTokenLifetime time.Duration `env:"AUTH_BEARER_LIFETIME" default:"15m"`
    // RefreshTokenLifetime はリフレッシュトークンの有効期間。
    RefreshTokenLifetime time.Duration `env:"AUTH_REFRESH_LIFETIME" default:"168h"` // 7日
}

// CSRFConfig は CSRF 保護の設定。
type CSRFConfig struct {
    // Enabled はCSRF保護の有効/無効。デフォルトは有効。
    Enabled bool `env:"CSRF_ENABLED" default:"true"`
    // CookieName はCSRFトークンの Cookie 名。
    CookieName string `env:"CSRF_COOKIE_NAME" default:"_broth_csrf"`
    // HeaderName はAJAXリクエストで送信するヘッダ名。
    HeaderName string `env:"CSRF_HEADER_NAME" default:"X-CSRF-Token"`
    // Secure は Cookie に Secure フラグを付与するか。本番環境では true。
    Secure bool `env:"CSRF_SECURE" default:"true"`
}

// CORSConfig は CORS の設定。
type CORSConfig struct {
    AllowedOrigins   []string `env:"CORS_ALLOWED_ORIGINS"` // カンマ区切り
    AllowedMethods   []string `env:"CORS_ALLOWED_METHODS"   default:"GET,POST,PUT,PATCH,DELETE"`
    AllowedHeaders   []string `env:"CORS_ALLOWED_HEADERS"   default:"Content-Type,Authorization,X-CSRF-Token"`
    AllowCredentials bool     `env:"CORS_ALLOW_CREDENTIALS" default:"false"`
    MaxAge           int      `env:"CORS_MAX_AGE"           default:"86400"` // 秒
}

// RateLimitConfig はレート制限の設定。
type RateLimitConfig struct {
    // GlobalRPS はアプリケーション全体の1IPあたりの毎秒リクエスト数上限。
    GlobalRPS int `env:"RATE_LIMIT_GLOBAL_RPS" default:"100"`
    // LoginMaxAttempts はログインエンドポイントの試行回数上限（ウィンドウあたり）。
    LoginMaxAttempts int `env:"RATE_LIMIT_LOGIN_MAX" default:"5"`
    // LoginWindow はログインのレート制限ウィンドウ。
    LoginWindow time.Duration `env:"RATE_LIMIT_LOGIN_WINDOW" default:"15m"`
}
```

---

## 3. コンテキスト自動適用（差別化機能）

**これが Broth の最重要セキュリティ差別化ポイントである。**

### 3.1 問題: 既存フレームワークの課題

既存の Go フレームワーク（Gin, Echo, Chi）では、SSR と API を同一アプリケーションで提供する場合、セキュリティ設定を手動で分岐させる必要がある。

```go
// 既存フレームワークでの問題: 手動でルートグループを分離する必要がある
ssrGroup := router.Group("/")
ssrGroup.Use(csrfMiddleware)           // SSRにはCSRF必要
ssrGroup.Use(sessionAuthMiddleware)    // SSRはセッション認証

apiGroup := router.Group("/api")
apiGroup.Use(corsMiddleware)           // APIにはCORS必要
apiGroup.Use(bearerAuthMiddleware)     // APIはBearer認証
// → ルートの配置を間違えると、CSRFなしのSSRや、セッション認証のAPIが生まれる
```

この手動分離は以下の問題を引き起こす。

- ルート配置のミスによるセキュリティ穴
- SSR/API 混在エンドポイント（HTMLとJSONの両方を返す）への対応が困難
- 新規エンドポイント追加時に「どのグループに入れるか」の判断が必要

### 3.2 Broth のアプローチ: リクエスト単位のコンテキスト判定

Broth はルートグループではなく **リクエスト単位** で SSR/API を判定し、適切なセキュリティスタックを自動適用する。

```mermaid
flowchart TD
    REQ["受信リクエスト"] --> DETECT{"ContextDetect<br/>ミドルウェア"}

    DETECT -->|"判定ロジック"| CHECK1{"Content-Type に<br/>application/json を含む？"}
    CHECK1 -->|Yes| API
    CHECK1 -->|No| CHECK2{"Accept に<br/>application/json を含む？"}
    CHECK2 -->|Yes| API
    CHECK2 -->|No| CHECK3{"Authorization:<br/>Bearer ヘッダあり？"}
    CHECK3 -->|Yes| API
    CHECK3 -->|No| CHECK4{"X-Requested-With:<br/>XMLHttpRequest？"}
    CHECK4 -->|Yes| API
    CHECK4 -->|No| SSR

    API["API コンテキスト"]
    SSR["SSR コンテキスト"]

    API --> API_STACK["CSRF: OFF<br/>認証: Bearer トークン<br/>エラー: JSON形式<br/>CORS: ON"]
    SSR --> SSR_STACK["CSRF: ON<br/>認証: セッション<br/>エラー: HTMLページ<br/>CORS: OFF"]

    style DETECT fill:#E74C3C,color:#fff
    style API fill:#4A90D9,color:#fff
    style SSR fill:#50B848,color:#fff
    style API_STACK fill:#4A90D9,color:#fff
    style SSR_STACK fill:#50B848,color:#fff
```

### 3.3 判定ロジックの実装

```go
// broth/middleware/context_detect.go
package middleware

import (
    "context"
    "net/http"
    "strings"
)

// RequestContext はリクエストの文脈（SSR or API）を表す。
type RequestContext int

const (
    // ContextSSR はブラウザからのSSRリクエストを示す。
    ContextSSR RequestContext = iota
    // ContextAPI はAPIクライアントからのリクエストを示す。
    ContextAPI
)

// String は RequestContext の文字列表現を返す。
func (rc RequestContext) String() string {
    switch rc {
    case ContextAPI:
        return "api"
    default:
        return "ssr"
    }
}

// requestContextKey はコンテキストに RequestContext を格納するためのキー。
type requestContextKey struct{}

// DetectRequestContext はリクエストヘッダから SSR/API を判定する。
// 判定ルール（優先度順）:
//   1. Content-Type: application/json → API
//   2. Accept: application/json（text/html を含まない場合）→ API
//   3. Authorization: Bearer → API
//   4. X-Requested-With: XMLHttpRequest → API
//   5. 上記いずれにも該当しない → SSR
func DetectRequestContext(r *http.Request) RequestContext {
    // 1. Content-Type チェック
    ct := r.Header.Get("Content-Type")
    if strings.Contains(ct, "application/json") {
        return ContextAPI
    }

    // 2. Accept チェック（text/html を同時に含む場合は SSR を優先）
    accept := r.Header.Get("Accept")
    if strings.Contains(accept, "application/json") &&
        !strings.Contains(accept, "text/html") {
        return ContextAPI
    }

    // 3. Authorization: Bearer チェック
    auth := r.Header.Get("Authorization")
    if strings.HasPrefix(auth, "Bearer ") {
        return ContextAPI
    }

    // 4. X-Requested-With チェック（AJAX）
    if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
        return ContextAPI
    }

    // 5. デフォルトは SSR（安全側に倒す）
    return ContextSSR
}

// GetRequestContext はコンテキストから RequestContext を取得する。
func GetRequestContext(ctx context.Context) RequestContext {
    if rc, ok := ctx.Value(requestContextKey{}).(RequestContext); ok {
        return rc
    }
    return ContextSSR // デフォルトは SSR（安全側に倒す）
}

// ContextDetect は SSR/API コンテキスト判定ミドルウェアを返す。
func ContextDetect() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            rc := DetectRequestContext(r)
            ctx := context.WithValue(r.Context(), requestContextKey{}, rc)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### 3.4 各ミドルウェアでの文脈参照

`ContextDetect` で判定された結果を、後続のミドルウェアが参照する。

```go
// broth/middleware/csrf.go（抜粋）
func CSRF(cfg CSRFConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            rc := GetRequestContext(r.Context())

            // API コンテキストでは CSRF チェックをスキップ
            if rc == ContextAPI {
                next.ServeHTTP(w, r)
                return
            }

            // SSR コンテキストでは CSRF チェックを実行
            // ...（後述の CSRF 実装）
            next.ServeHTTP(w, r)
        })
    }
}
```

```go
// broth/middleware/auth.go（抜粋）
func Auth(cfg AuthConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            rc := GetRequestContext(r.Context())

            var user *auth.User
            var err error

            switch rc {
            case ContextAPI:
                // Bearer トークンから認証
                user, err = authenticateBearer(r, cfg)
            case ContextSSR:
                // セッションから認証
                user, err = authenticateSession(r, cfg)
            }

            if err != nil {
                handleAuthError(w, r, rc, err)
                return
            }

            // 認証済みユーザーをコンテキストに設定
            if user != nil {
                ctx := auth.SetUser(r.Context(), user)
                r = r.WithContext(ctx)
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

### 3.5 エラーレスポンスの文脈切替

```go
// broth/middleware/error.go
package middleware

import (
    "encoding/json"
    "net/http"

    "github.com/source-maker/broth/render"
)

// HandleError は文脈に応じたエラーレスポンスを返す。
func HandleError(w http.ResponseWriter, r *http.Request, status int, err error) {
    rc := GetRequestContext(r.Context())

    switch rc {
    case ContextAPI:
        // JSON エラーレスポンス
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(status)
        json.NewEncoder(w).Encode(map[string]any{
            "error":   http.StatusText(status),
            "message": err.Error(),
        })
    case ContextSSR:
        // HTML エラーページ
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.WriteHeader(status)
        render.ErrorPage(w, status, err)
    }
}
```

### 3.6 「魔法的すぎないか」の検討と判断根拠

コンテキスト自動適用は強力だが「暗黙の動作」であり、Go の明示性の文化に反する可能性がある。以下に検討結果を記録する。

| 懸念 | 検討結果 | 対策 |
|---|---|---|
| **判定ロジックが予測不能** | 判定ルールは5項目のみで、全てHTTPヘッダの静的チェック。副作用なし | 判定ルールをドキュメント化し、`broth routes` コマンドで各ルートのデフォルト文脈を表示 |
| **意図しないCSRF無効化** | APIと誤判定されるとCSRFが無効になる | デフォルトをSSR（安全側）に倒す。判定結果をログに出力 |
| **デバッグが困難** | 判定結果が見えない | `X-Broth-Context: ssr` / `X-Broth-Context: api` レスポンスヘッダを開発モードで出力 |
| **明示的な制御ができない** | 自動判定を上書きしたい場合がある | ルート単位のオーバーライド機構を提供 |

**判断**: 以下の理由で「許容できる魔法」と判断する。

1. **安全側に倒す設計**: 判定不能時はSSR（CSRF ON）をデフォルトとする。誤判定があっても「安全すぎる」方向に倒れる
2. **透明性の担保**: 開発モードでのヘッダ出力、ログへの判定結果記録、`broth routes` での一覧表示
3. **オプトアウト可能**: ルート単位・ミドルウェア単位で明示的にオーバーライドできる
4. **Go のイディオムとの整合**: `context.Context` による値伝搬は Go の標準的パターンであり、ミドルウェア間の情報伝達に広く使われている

### 3.7 ルート単位のオーバーライド

自動判定をオーバーライドしたい場合に、明示的にコンテキストを指定できる。

```go
// modules/account/routes.go
func (h *Handler) Routes() []router.Route {
    return []router.Route{
        // 通常のSSRルート（自動判定に任せる）
        {Pattern: "GET /login", Handler: http.HandlerFunc(h.ShowLoginForm)},
        {Pattern: "POST /login", Handler: http.HandlerFunc(h.Login)},

        // 明示的にAPIコンテキストを強制（SSRプロジェクトだがJSONも返すエンドポイント）
        {Pattern: "GET /api/me", Handler: middleware.ForceContext(
            ContextAPI,
            http.HandlerFunc(h.APICurrentUser),
        )},

        // 明示的にSSRコンテキストを強制（API経由でもCSRFを強制したい場合）
        {Pattern: "POST /sensitive-action", Handler: middleware.ForceContext(
            ContextSSR,
            http.HandlerFunc(h.SensitiveAction),
        )},
    }
}
```

```go
// broth/middleware/context_detect.go（追加）

// ForceContext は特定のルートに対してコンテキストを強制する。
func ForceContext(rc RequestContext, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := context.WithValue(r.Context(), requestContextKey{}, rc)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 3.8 SSR/API 文脈の自動適用マトリックス

| セキュリティ機能 | SSR（ブラウザ） | API（Bearer/JSON） | 根拠 |
|---|---|---|---|
| **CSRF保護** | ON（自動） | OFF | ブラウザのCookieベース攻撃はAPI（Bearerトークン）では発生しない |
| **認証方式** | セッション認証 | Bearerトークン認証 | ブラウザはCookieを自動送信、APIクライアントはAuthorizationヘッダを明示送信 |
| **エラーレスポンス** | HTMLエラーページ | JSONエラーレスポンス | レスポンスの Content-Type を文脈に合わせる |
| **CORS** | OFF | ON（設定ベース） | SSRは同一オリジン、APIはクロスオリジンからのアクセスを許可する場合がある |
| **セキュリティヘッダ** | 全て適用 | CSP を緩和 | APIはブラウザレンダリングされないため、CSP は不要 |
| **SameSite Cookie** | `Lax` | 適用されない | APIはCookieを使用しない |

---

## 4. 認証（AuthN）設計

### 4.1 broth/auth パッケージの API 設計

ARCHITECTURE.md sec 4 で定義された `broth/auth` パッケージの設計。横断的関心事として全レイヤーから参照可能。

```go
// broth/auth/user.go
package auth

import "context"

// User は認証済みユーザーの情報を保持する。
// 認証方式（セッション/Bearer）に依存しない統一的な型。
type User struct {
    ID    int64
    Email string
    Name  string
    Roles []string // ロールベースアクセス制御用
}

// 型安全なコンテキストキー（ARCHITECTURE.md sec 3.1 の ctxKey パターンに準拠）
type userContextKey struct{}

// SetUser はコンテキストに認証済みユーザーを設定する。
func SetUser(ctx context.Context, user *User) context.Context {
    return context.WithValue(ctx, userContextKey{}, user)
}

// GetUser はコンテキストから認証済みユーザーを取得する。
// 未認証の場合は nil を返す。
func GetUser(ctx context.Context) *User {
    user, _ := ctx.Value(userContextKey{}).(*User)
    return user
}

// MustGetUser はコンテキストから認証済みユーザーを取得する。
// 未認証の場合はパニックする（認証必須のハンドラ内で使用）。
func MustGetUser(ctx context.Context) *User {
    user := GetUser(ctx)
    if user == nil {
        panic("auth: user not found in context (ensure Auth middleware is applied)")
    }
    return user
}

// IsAuthenticated はユーザーが認証済みかどうかを返す。
func IsAuthenticated(ctx context.Context) bool {
    return GetUser(ctx) != nil
}
```

### 4.2 セッションベース認証（SSR向け）

#### セッションストア

`broth/session` パッケージが複数のバックエンドをサポートする。

```go
// broth/session/store.go
package session

import (
    "context"
    "time"
)

// Store はセッションデータの永続化を抽象化する。
type Store interface {
    // Get はセッションIDからセッションデータを取得する。
    Get(ctx context.Context, id string) (*Session, error)
    // Save はセッションデータを保存する。
    Save(ctx context.Context, session *Session) error
    // Delete はセッションを削除する。
    Delete(ctx context.Context, id string) error
    // GC は期限切れのセッションを削除する。
    GC(ctx context.Context) error
}

// Session はセッションデータを表す。
type Session struct {
    ID        string
    Data      map[string]any
    UserID    int64 // 認証済みの場合に設定
    CreatedAt time.Time
    ExpiresAt time.Time
    IsNew     bool
}
```

```go
// broth/session/cookie_store.go -- Cookie バックエンド（最小構成・デフォルト）
package session

import (
    "crypto/aes"
    "crypto/cipher"
)

// CookieStore はセッションデータを暗号化してCookieに保存する。
// DB不要で最も軽量。ただしセッションサイズは4KBに制限される。
type CookieStore struct {
    gcm       cipher.AEAD
    secretKey []byte
}

// NewCookieStore は CookieStore を生成する。
func NewCookieStore(secretKey []byte) (*CookieStore, error) {
    block, err := aes.NewCipher(secretKey[:32])
    if err != nil {
        return nil, err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    return &CookieStore{gcm: gcm, secretKey: secretKey}, nil
}
```

```go
// broth/session/db_store.go -- DB バックエンド（標準構成）
package session

import "database/sql"

// DBStore はセッションデータをデータベースに保存する。
// スケーラビリティと機能のバランスが良い標準的な選択。
type DBStore struct {
    db *sql.DB
}

// NewDBStore は DBStore を生成する。
// 以下のテーブルが必要:
//   CREATE TABLE sessions (
//       id         TEXT PRIMARY KEY,
//       data       BYTEA NOT NULL,
//       user_id    BIGINT,
//       created_at TIMESTAMPTZ NOT NULL,
//       expires_at TIMESTAMPTZ NOT NULL
//   );
//   CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);
//   CREATE INDEX idx_sessions_user_id ON sessions (user_id);
func NewDBStore(db *sql.DB) *DBStore {
    return &DBStore{db: db}
}
```

#### ログイン/ログアウトフロー

```mermaid
sequenceDiagram
    participant B as ブラウザ
    participant MW as Middleware<br/>(Session + Auth)
    participant H as Handler
    participant S as Service
    participant DB as Database

    Note over B,DB: ログインフロー

    B->>MW: POST /login (email, password, remember_me)
    MW->>H: セッション復元（未認証）
    H->>S: Authenticate(ctx, LoginInput)
    S->>DB: FindByEmail(email)
    DB-->>S: User
    S->>S: user.CheckPassword(password)
    S-->>H: *User, nil

    Note over H: セッション固定攻撃防止:<br/>既存セッションIDを破棄し<br/>新しいセッションIDを生成

    H->>MW: セッションにUserID設定 + 新セッションID生成
    MW->>B: Set-Cookie: _broth_session=<new_id>; HttpOnly; Secure; SameSite=Lax

    Note over B,DB: ログアウトフロー

    B->>MW: POST /logout
    MW->>H: セッション復元（認証済み）
    H->>MW: セッション破棄
    MW->>DB: DELETE FROM sessions WHERE id = ?
    MW->>B: Set-Cookie: _broth_session=; Max-Age=0
```

#### ログインのコード例

```go
// modules/account/handler.go
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
    var input LoginInput
    if err := parseForm(r, &input); err != nil {
        h.renderer.Error(w, r, http.StatusBadRequest, err)
        return
    }

    user, err := h.svc.Authenticate(r.Context(), input)
    if err != nil {
        h.renderer.Error(w, r, http.StatusUnauthorized, err)
        return
    }

    // セッション固定攻撃防止: 新しいセッションIDで再生成
    sess := session.FromContext(r.Context())
    sess.Regenerate() // 既存セッションIDを無効化し、新IDを生成
    sess.Set("user_id", user.ID)

    // Remember Me
    if input.RememberMe {
        sess.SetMaxAge(30 * 24 * 60 * 60) // 30日
    }

    http.Redirect(w, r, "/", http.StatusSeeOther)
}

// modules/account/handler.go
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
    sess := session.FromContext(r.Context())
    sess.Destroy() // セッションデータをストアから削除し、Cookieを無効化

    http.Redirect(w, r, "/login", http.StatusSeeOther)
}
```

#### Remember Me 機能

```go
// broth/session/session.go（追加メソッド）

// SetMaxAge はセッションの有効期間を設定する。
// Remember Me の場合は長い期間を設定する。
func (s *Session) SetMaxAge(seconds int) {
    s.ExpiresAt = time.Now().Add(time.Duration(seconds) * time.Second)
    s.rememberMe = true
}
```

Remember Me の Cookie 設定:

| 属性 | 通常セッション | Remember Me |
|---|---|---|
| `Max-Age` | 省略（ブラウザセッション） | 2,592,000（30日） |
| `HttpOnly` | `true` | `true` |
| `Secure` | `true`（本番） | `true`（本番） |
| `SameSite` | `Lax` | `Lax` |
| `Path` | `/` | `/` |

#### セッション固定攻撃防止

ログイン時に `sess.Regenerate()` を呼び出すことで、以下の手順でセッション固定攻撃を防止する。

1. 現在のセッションIDをストアから削除
2. 新しいセッションIDを `crypto/rand` で生成
3. セッションデータを新しいIDで保存
4. 新しいIDの Cookie をレスポンスに設定

```go
// broth/session/session.go
func (s *Session) Regenerate() {
    oldID := s.ID
    s.ID = generateSecureID() // crypto/rand ベース
    s.regenerated = true
    s.oldID = oldID // ストアからの削除用
}
```

### 4.3 JWT/Bearer トークン認証（API向け）

#### トークン設計

```mermaid
sequenceDiagram
    participant C as APIクライアント
    participant MW as Auth Middleware
    participant H as Handler
    participant S as Service
    participant DB as Database

    Note over C,DB: トークン取得フロー

    C->>H: POST /api/auth/token (email, password)
    H->>S: Authenticate(ctx, LoginInput)
    S->>DB: FindByEmail + CheckPassword
    S-->>H: *User

    Note over H: アクセストークン（短命）+<br/>リフレッシュトークン（長命）を生成

    H-->>C: {"access_token": "...", "refresh_token": "...", "expires_in": 900}

    Note over C,DB: API リクエストフロー

    C->>MW: GET /api/articles<br/>Authorization: Bearer <access_token>
    MW->>MW: トークン検証（署名・有効期限）
    MW->>H: 認証済みユーザーをコンテキストに設定
    H-->>C: 200 OK

    Note over C,DB: トークンリフレッシュフロー

    C->>H: POST /api/auth/refresh<br/>{"refresh_token": "..."}
    H->>DB: リフレッシュトークンの検証・取得
    H->>H: 新しいアクセストークンを生成
    H->>DB: 古いリフレッシュトークンを無効化（ローテーション）
    H-->>C: {"access_token": "...", "refresh_token": "...", "expires_in": 900}
```

#### トークン設計の詳細

| 項目 | アクセストークン | リフレッシュトークン |
|---|---|---|
| **形式** | HMAC-SHA256 署名付き JWT | `crypto/rand` で生成したランダムトークン |
| **有効期限** | 15分（デフォルト） | 7日（デフォルト） |
| **保存場所** | クライアント側のみ（サーバー側で保持しない） | データベース |
| **無効化** | 有効期限切れのみ（ステートレス） | DB から削除でき即座に無効化可能 |
| **用途** | API リクエストの認証 | アクセストークンの再発行 |

```go
// broth/auth/token.go
package auth

import (
    "crypto/rand"
    "encoding/base64"
    "errors"
    "fmt"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

// TokenPair はアクセストークンとリフレッシュトークンのペア。
type TokenPair struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int    `json:"expires_in"` // 秒
    TokenType    string `json:"token_type"` // "Bearer"
}

// TokenClaims はアクセストークンに含まれるカスタムクレーム。
// jwt.RegisteredClaims を埋め込み、標準クレーム（exp, iat 等）の処理をライブラリに委譲する。
type TokenClaims struct {
    jwt.RegisteredClaims
    UserID int64    `json:"uid"`
    Email  string   `json:"email"`
    Roles  []string `json:"roles,omitempty"`
}

// TokenService はトークンの生成と検証を行う。
type TokenService struct {
    secretKey      []byte
    accessLifetime time.Duration
}

// NewTokenService は TokenService を生成する。
func NewTokenService(secretKey []byte, accessLifetime time.Duration) *TokenService {
    return &TokenService{
        secretKey:      secretKey,
        accessLifetime: accessLifetime,
    }
}

// GenerateAccessToken はアクセストークン（JWT）を生成する。
// golang-jwt/jwt を使用し、HMAC-SHA256 で署名する。
func (ts *TokenService) GenerateAccessToken(user *User) (string, error) {
    now := time.Now()
    claims := TokenClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(ts.accessLifetime)),
        },
        UserID: user.ID,
        Email:  user.Email,
        Roles:  user.Roles,
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(ts.secretKey)
}

// ValidateAccessToken はアクセストークンを検証し、クレームを返す。
// 署名検証・有効期限チェックは golang-jwt/jwt が自動的に行う。
// 署名アルゴリズムの明示的検証により、alg=none 攻撃を防止する。
func (ts *TokenService) ValidateAccessToken(tokenStr string) (*TokenClaims, error) {
    token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{},
        func(t *jwt.Token) (interface{}, error) {
            // 署名アルゴリズムの検証（alg=none / RS256 混同攻撃の防止）
            if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("auth: unexpected signing method: %v", t.Header["alg"])
            }
            return ts.secretKey, nil
        },
    )
    if err != nil {
        return nil, fmt.Errorf("auth: token validation failed: %w", err)
    }
    claims, ok := token.Claims.(*TokenClaims)
    if !ok || !token.Valid {
        return nil, errors.New("auth: invalid token claims")
    }
    return claims, nil
}

// GenerateRefreshToken はリフレッシュトークン（ランダム文字列）を生成する。
// リフレッシュトークンは DB に保存する。
func GenerateRefreshToken() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(b), nil
}
```

#### トークン検証ミドルウェア

```go
// broth/middleware/auth.go（Bearer認証部分）
func authenticateBearer(r *http.Request, cfg AuthConfig) (*auth.User, error) {
    header := r.Header.Get("Authorization")
    if header == "" {
        return nil, nil // 未認証（エラーではない）
    }

    if !strings.HasPrefix(header, "Bearer ") {
        return nil, errors.New("auth: invalid authorization header format")
    }

    token := strings.TrimPrefix(header, "Bearer ")
    claims, err := cfg.TokenService.ValidateAccessToken(token)
    if err != nil {
        return nil, fmt.Errorf("auth: token validation failed: %w", err)
    }

    return &auth.User{
        ID:    claims.UserID,
        Email: claims.Email,
        Roles: claims.Roles,
    }, nil
}
```

### 4.4 OAuth2 連携の拡張ポイント

```go
// broth/auth/oauth2.go
package auth

import (
    "context"
    "net/http"
)

// OAuth2UserInfo は OAuth2 プロバイダから取得したユーザー情報。
type OAuth2UserInfo struct {
    ProviderID   string // プロバイダ固有のユーザーID
    ProviderName string // "google", "github" 等
    Email        string
    Name         string
    AvatarURL    string
}

// OAuth2Provider は OAuth2 認証プロバイダのインターフェース。
type OAuth2Provider interface {
    // Name はプロバイダ名を返す（"google", "github" 等）。
    Name() string

    // AuthURL は認証URLを生成する。
    // state パラメータは CSRF 対策として使用する。
    AuthURL(state string) string

    // Exchange は認可コードをアクセストークンに交換し、ユーザー情報を取得する。
    Exchange(ctx context.Context, code string) (*OAuth2UserInfo, error)
}

// OAuth2Handler は OAuth2 コールバックの標準ハンドラ。
type OAuth2Handler struct {
    providers map[string]OAuth2Provider
    onLogin   func(ctx context.Context, info *OAuth2UserInfo) (*User, error)
}

// NewOAuth2Handler は OAuth2Handler を生成する。
// onLogin は OAuth2 ユーザー情報からアプリケーションのユーザーを解決するコールバック。
func NewOAuth2Handler(
    onLogin func(ctx context.Context, info *OAuth2UserInfo) (*User, error),
    providers ...OAuth2Provider,
) *OAuth2Handler {
    pm := make(map[string]OAuth2Provider)
    for _, p := range providers {
        pm[p.Name()] = p
    }
    return &OAuth2Handler{providers: pm, onLogin: onLogin}
}

// HandleCallback は OAuth2 コールバックを処理する標準ハンドラ。
// GET /auth/{provider}/callback?code=xxx&state=xxx
func (h *OAuth2Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
    providerName := r.PathValue("provider")
    provider, ok := h.providers[providerName]
    if !ok {
        http.Error(w, "unknown provider", http.StatusBadRequest)
        return
    }

    // state の検証（CSRF対策）
    // code → token → userinfo の交換
    // onLogin コールバックでユーザー解決
    // セッションへのユーザーID設定
    _ = provider // 実装省略
}
```

```go
// 使用例: Google OAuth2 プロバイダの実装
// broth/auth/providers/google.go
package providers

import "github.com/source-maker/broth/auth"

type GoogleProvider struct {
    clientID     string
    clientSecret string
    redirectURL  string
}

func NewGoogle(clientID, clientSecret, redirectURL string) *GoogleProvider {
    return &GoogleProvider{
        clientID:     clientID,
        clientSecret: clientSecret,
        redirectURL:  redirectURL,
    }
}

func (g *GoogleProvider) Name() string { return "google" }

// AuthURL, Exchange は OAuth2Provider インターフェースの実装
// ...
```

### 4.5 パスワードハッシュ

```go
// broth/auth/password.go
package auth

import (
    "crypto/subtle"
    "errors"
    "fmt"

    "golang.org/x/crypto/bcrypt"
)

// PasswordHasher はパスワードハッシュの生成と検証を行う。
type PasswordHasher interface {
    Hash(password string) (string, error)
    Verify(password, hash string) error
}

// BcryptHasher は bcrypt ベースのパスワードハッシュ。
// デフォルトのハッシュアルゴリズム。
type BcryptHasher struct {
    cost int
}

// NewBcryptHasher は BcryptHasher を生成する。
// cost は bcrypt のコスト（デフォルト: bcrypt.DefaultCost = 10）。
func NewBcryptHasher(cost int) *BcryptHasher {
    if cost == 0 {
        cost = bcrypt.DefaultCost
    }
    return &BcryptHasher{cost: cost}
}

// Hash はパスワードを bcrypt でハッシュ化する。
func (h *BcryptHasher) Hash(password string) (string, error) {
    if len(password) < 8 {
        return "", errors.New("auth: password must be at least 8 characters")
    }
    hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
    if err != nil {
        return "", fmt.Errorf("auth: bcrypt hash: %w", err)
    }
    return string(hash), nil
}

// Verify はパスワードとハッシュを検証する。
func (h *BcryptHasher) Verify(password, hash string) error {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// デフォルトのパスワードハッシャー
var DefaultPasswordHasher PasswordHasher = NewBcryptHasher(0)
```

**設計判断 -- bcrypt vs argon2**:
- **デフォルト**: bcrypt を採用。Go 標準の `golang.org/x/crypto/bcrypt` で利用可能で、サードパーティ依存が最小
- **argon2 は将来オプションとして提供**: argon2id はメモリハード関数としてより安全だが、パラメータ調整が複雑でデフォルトとしては bcrypt が適切
- ARCHITECTURE.md の「サードパーティ依存は最小限」原則に合致

---

## 5. 認可（AuthZ）設計

### 5.1 ロールベースアクセス制御（RBAC）

#### Role/Permission モデル

```go
// broth/auth/rbac.go
package auth

// Role はユーザーに割り当てられるロール。
type Role struct {
    Name        string
    Permissions []Permission
}

// Permission は操作の権限を表す。
// "resource:action" の形式（例: "article:create", "user:delete"）。
type Permission string

// 標準的なロール定義例
const (
    RoleAdmin  = "admin"
    RoleEditor = "editor"
    RoleViewer = "viewer"
)

// 標準的なパーミッション
const (
    PermCreate Permission = "create"
    PermRead   Permission = "read"
    PermUpdate Permission = "update"
    PermDelete Permission = "delete"
)

// HasRole はユーザーが指定のロールを持っているかを検証する。
func (u *User) HasRole(role string) bool {
    for _, r := range u.Roles {
        if r == role {
            return true
        }
    }
    return false
}
```

#### ミドルウェアでの認可チェック

```go
// broth/middleware/authorize.go
package middleware

import (
    "net/http"

    "github.com/source-maker/broth/auth"
)

// RequireAuth は認証済みであることを要求するミドルウェア。
// 未認証の場合は SSR なら /login にリダイレクト、API なら 401 を返す。
func RequireAuth() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !auth.IsAuthenticated(r.Context()) {
                rc := GetRequestContext(r.Context())
                switch rc {
                case ContextSSR:
                    http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusSeeOther)
                case ContextAPI:
                    HandleError(w, r, http.StatusUnauthorized,
                        errors.New("authentication required"))
                }
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// RequireRole は特定のロールを要求するミドルウェア。
func RequireRole(roles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user := auth.GetUser(r.Context())
            if user == nil {
                HandleError(w, r, http.StatusUnauthorized,
                    errors.New("authentication required"))
                return
            }

            for _, role := range roles {
                if user.HasRole(role) {
                    next.ServeHTTP(w, r)
                    return
                }
            }

            HandleError(w, r, http.StatusForbidden,
                errors.New("insufficient permissions"))
        })
    }
}
```

### 5.2 リソースベースポリシー

「このユーザーがこのリソースを操作できるか」の判定パターン。

```go
// broth/auth/policy.go
package auth

import "context"

// Action はリソースに対する操作を表す。
type Action string

const (
    ActionView   Action = "view"
    ActionCreate Action = "create"
    ActionUpdate Action = "update"
    ActionDelete Action = "delete"
)

// Policy はリソースベースの認可ポリシーを定義するインターフェース。
// 各モジュールがドメイン固有のポリシーを実装する。
type Policy[T any] interface {
    // Authorize はユーザーがリソースに対して操作可能かを判定する。
    // 許可される場合は nil を返し、拒否される場合は error を返す。
    Authorize(ctx context.Context, user *User, action Action, resource T) error
}
```

#### モジュールでのポリシー実装例

```go
// modules/article/policy.go
package article

import (
    "context"
    "errors"

    "github.com/source-maker/broth/auth"
)

// ArticlePolicy は記事に対する認可ポリシー。
type ArticlePolicy struct{}

// Authorize は記事に対する操作の認可を判定する。
func (p *ArticlePolicy) Authorize(
    ctx context.Context,
    user *auth.User,
    action auth.Action,
    article *Article,
) error {
    switch action {
    case auth.ActionView:
        // 公開記事は誰でも閲覧可能
        if article.IsPublished {
            return nil
        }
        // 非公開記事は著者本人またはadminのみ
        if user != nil && (user.ID == article.AuthorID || user.HasRole(auth.RoleAdmin)) {
            return nil
        }
        return errors.New("article: not authorized to view this article")

    case auth.ActionUpdate:
        // 著者本人またはeditor以上
        if user == nil {
            return errors.New("article: authentication required")
        }
        if user.ID == article.AuthorID || user.HasRole(auth.RoleEditor) || user.HasRole(auth.RoleAdmin) {
            return nil
        }
        return errors.New("article: not authorized to update this article")

    case auth.ActionDelete:
        // adminのみ、または著者本人
        if user == nil {
            return errors.New("article: authentication required")
        }
        if user.ID == article.AuthorID || user.HasRole(auth.RoleAdmin) {
            return nil
        }
        return errors.New("article: not authorized to delete this article")

    default:
        return errors.New("article: unknown action")
    }
}

// インターフェース適合チェック
var _ auth.Policy[*Article] = (*ArticlePolicy)(nil)
```

### 5.3 認可チェックの標準パターン

#### パターン1: ミドルウェアでのルート単位の認可

```go
// modules/account/routes.go
func (h *Handler) Routes() []router.Route {
    return []router.Route{
        // 認証不要
        {Pattern: "GET /login", Handler: http.HandlerFunc(h.ShowLoginForm)},
        {Pattern: "POST /login", Handler: http.HandlerFunc(h.Login)},

        // 認証必須（RequireAuth ミドルウェアでルート単位に適用）
        {Pattern: "GET /profile", Handler: middleware.RequireAuth()(
            http.HandlerFunc(h.ShowProfile),
        )},

        // 特定ロール必須
        {Pattern: "GET /admin/users", Handler: middleware.RequireRole(auth.RoleAdmin)(
            http.HandlerFunc(h.AdminUserList),
        )},
    }
}
```

#### パターン2: ハンドラ内でのリソース単位の認可

```go
// modules/article/handler.go
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
    // 1. リソースの取得
    id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
    article, err := h.svc.FindByID(r.Context(), id)
    if err != nil {
        HandleError(w, r, http.StatusNotFound, err)
        return
    }

    // 2. リソースベースの認可チェック
    user := auth.GetUser(r.Context())
    if err := h.policy.Authorize(r.Context(), user, auth.ActionUpdate, article); err != nil {
        HandleError(w, r, http.StatusForbidden, err)
        return
    }

    // 3. 認可OKなら処理を続行
    h.renderer.HTML(w, r, http.StatusOK, "article/edit.html", map[string]any{
        "Article": article,
    })
}
```

#### パターン3: サービス層での認可チェック（推奨パターン）

ビジネスルールとして認可が必要な場合は、サービス層で判定する。

```go
// modules/article/service.go
func (s *Service) Update(ctx context.Context, user *auth.User, input UpdateInput) (*Article, error) {
    article, err := s.repo.FindByID(ctx, input.ID)
    if err != nil {
        return nil, fmt.Errorf("article: not found: %w", err)
    }

    // 認可チェック（サービス層で実行）
    if err := s.policy.Authorize(ctx, user, auth.ActionUpdate, article); err != nil {
        return nil, fmt.Errorf("article: %w", err)
    }

    // ビジネスロジック
    article.Title = input.Title
    article.Body = input.Body
    article.UpdatedAt = time.Now()

    if err := s.repo.Update(ctx, article); err != nil {
        return nil, fmt.Errorf("article: update: %w", err)
    }

    return article, nil
}
```

---

## 6. 基本防御

### 6.1 CSRF 保護

#### パターン選択: Synchronizer Token パターン

**決定**: Synchronizer Token パターンを採用する（ダブルサブミットCookieパターンではなく）。

**根拠**:

| 観点 | Synchronizer Token | ダブルサブミットCookie |
|---|---|---|
| **セキュリティ強度** | サーバー側で検証。より堅牢 | Cookie 操作可能なサブドメイン攻撃に脆弱 |
| **実装の複雑さ** | セッションストアが必要 | ステートレスで単純 |
| **Broth との相性** | セッション基盤が既にある | セッション基盤と重複する |
| **Django との一致** | Django と同じパターン | -- |

Broth は `broth/session` によるセッション基盤を前提としているため、Synchronizer Token パターンの実装コストは低い。セキュリティ強度を優先する。

#### 実装

```go
// broth/middleware/csrf.go
package middleware

import (
    "crypto/rand"
    "crypto/subtle"
    "encoding/base64"
    "errors"
    "net/http"

    "github.com/source-maker/broth/session"
)

const (
    csrfTokenLength = 32
    csrfSessionKey  = "_broth_csrf_token"
)

// CSRF は CSRF 保護ミドルウェアを返す。
// SSRコンテキストでのみ有効。APIコンテキストでは自動的にスキップされる。
func CSRF(cfg CSRFConfig) func(http.Handler) http.Handler {
    if !cfg.Enabled {
        return func(next http.Handler) http.Handler { return next }
    }

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // APIコンテキストではCSRFチェックをスキップ（sec 3 の文脈自動適用）
            rc := GetRequestContext(r.Context())
            if rc == ContextAPI {
                next.ServeHTTP(w, r)
                return
            }

            sess := session.FromContext(r.Context())

            // セッションにCSRFトークンがなければ生成
            token, ok := sess.Get(csrfSessionKey).(string)
            if !ok || token == "" {
                var err error
                token, err = generateCSRFToken()
                if err != nil {
                    http.Error(w, "Internal Server Error", http.StatusInternalServerError)
                    return
                }
                sess.Set(csrfSessionKey, token)
            }

            // トークンをコンテキストに設定（テンプレートから参照用）
            ctx := setCSRFToken(r.Context(), token)
            r = r.WithContext(ctx)

            // 安全なメソッド（GET, HEAD, OPTIONS, TRACE）はチェックをスキップ
            if isSafeMethod(r.Method) {
                next.ServeHTTP(w, r)
                return
            }

            // リクエストからトークンを取得（フォーム or ヘッダ）
            requestToken := r.FormValue("_csrf_token")
            if requestToken == "" {
                requestToken = r.Header.Get(cfg.HeaderName)
            }

            // 定数時間比較で検証
            if subtle.ConstantTimeCompare([]byte(token), []byte(requestToken)) != 1 {
                HandleError(w, r, http.StatusForbidden,
                    errors.New("CSRF token validation failed"))
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

// generateCSRFToken は暗号学的に安全なCSRFトークンを生成する。
func generateCSRFToken() (string, error) {
    b := make([]byte, csrfTokenLength)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(b), nil
}

func isSafeMethod(method string) bool {
    switch method {
    case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
        return true
    }
    return false
}
```

#### テンプレート統合

Django の `{% csrf_token %}` に相当するテンプレート関数を `broth/render` で提供する。

```go
// broth/render/helpers.go
package render

import (
    "html/template"
    "net/http"
)

// DefaultFuncMap はテンプレートで使用可能なヘルパー関数群を返す。
func DefaultFuncMap() template.FuncMap {
    return template.FuncMap{
        // csrfField は hidden input タグとして CSRF トークンを埋め込む。
        // 使用例: {{csrfField .}}
        "csrfField": func(r *http.Request) template.HTML {
            token := middleware.GetCSRFToken(r.Context())
            return template.HTML(`<input type="hidden" name="_csrf_token" value="` +
                template.HTMLEscapeString(token) + `">`)
        },
        // csrfToken はCSRFトークンの文字列を返す（AJAX用）。
        // 使用例: <meta name="csrf-token" content="{{csrfToken .}}">
        "csrfToken": func(r *http.Request) string {
            return middleware.GetCSRFToken(r.Context())
        },
    }
}
```

```html
<!-- templates/layouts/base.html -->
<html>
<head>
    <!-- AJAX用にmetaタグでCSRFトークンを公開 -->
    <meta name="csrf-token" content="{{csrfToken .Request}}">
</head>
<body>
    <!-- フォームでの使用例 -->
    <form method="POST" action="/login">
        {{csrfField .Request}}
        <input type="email" name="email">
        <input type="password" name="password">
        <button type="submit">ログイン</button>
    </form>
</body>
</html>
```

#### AJAX対応

```javascript
// static/js/app.js
// CSRFトークンをmetaタグから取得し、全てのfetch/XHR に自動付与
const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content;

// fetch のラッパー
function ogiRequest(url, options = {}) {
    const headers = options.headers || {};
    if (csrfToken && !['GET', 'HEAD', 'OPTIONS'].includes((options.method || 'GET').toUpperCase())) {
        headers['X-CSRF-Token'] = csrfToken;
    }
    return fetch(url, { ...options, headers });
}
```

#### SameSite Cookie との組み合わせ

CSRF対策の多層防御として、セッションCookieに `SameSite=Lax` を設定する。

| SameSite 値 | CSRF保護 | ユーザビリティ | Broth のデフォルト |
|---|---|---|---|
| `Strict` | 最強 | 外部サイトからのリンクでセッションが切れる | 採用しない |
| `Lax` | 強い（POST は保護される） | 通常のリンクは動作する | **採用** |
| `None` | なし | クロスサイト送信可能 | 採用しない |

`SameSite=Lax` + Synchronizer Token の組み合わせにより、二重の防御を提供する。

### 6.2 XSS 対策

#### html/template の auto-escape 活用

Go の `html/template` はデフォルトでコンテキスト依存の自動エスケープを行う。Broth は `broth/render` でこの機能を最大限活用する。

```go
// broth/render パッケージは html/template を使用
// テンプレート変数は自動的にエスケープされる
// {{.UserName}}  → &lt;script&gt;alert(1)&lt;/script&gt; に変換される
```

**安全でないHTMLの挿入が必要な場合**:

```go
// 明示的に template.HTML 型を使う場合のみエスケープが無効になる
// この場合、開発者が安全性を保証する責任を負う
data := map[string]any{
    "SafeHTML": template.HTML(sanitizedContent), // 明示的なオプトイン
}
```

#### Content-Security-Policy（CSP）

```go
// broth/middleware/security_headers.go（CSP 部分）

// デフォルトのCSP設定
// インラインスクリプト・スタイルを禁止し、XSSの影響を最小化する
const defaultCSP = "default-src 'self'; " +
    "script-src 'self'; " +
    "style-src 'self' 'unsafe-inline'; " + // CSSはインライン許可（実用性のため）
    "img-src 'self' data:; " +
    "font-src 'self'; " +
    "connect-src 'self'; " +
    "frame-ancestors 'none'; " +            // クリックジャッキング防止
    "base-uri 'self'; " +
    "form-action 'self'"
```

### 6.3 CORS

```go
// broth/middleware/cors.go
package middleware

import (
    "net/http"
    "strconv"
    "strings"
)

// CORS は CORS ミドルウェアを返す。
// APIコンテキストでのみ CORS ヘッダを設定する。
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            rc := GetRequestContext(r.Context())

            // SSRコンテキストではCORSヘッダを設定しない
            if rc == ContextSSR {
                next.ServeHTTP(w, r)
                return
            }

            origin := r.Header.Get("Origin")
            if origin == "" {
                next.ServeHTTP(w, r)
                return
            }

            // オリジンの検証
            if isAllowedOrigin(origin, cfg.AllowedOrigins) {
                w.Header().Set("Access-Control-Allow-Origin", origin)
                w.Header().Set("Vary", "Origin")

                if cfg.AllowCredentials {
                    w.Header().Set("Access-Control-Allow-Credentials", "true")
                }
            }

            // プリフライトリクエスト
            if r.Method == http.MethodOptions {
                w.Header().Set("Access-Control-Allow-Methods",
                    strings.Join(cfg.AllowedMethods, ", "))
                w.Header().Set("Access-Control-Allow-Headers",
                    strings.Join(cfg.AllowedHeaders, ", "))
                w.Header().Set("Access-Control-Max-Age",
                    strconv.Itoa(cfg.MaxAge))
                w.WriteHeader(http.StatusNoContent)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

func isAllowedOrigin(origin string, allowed []string) bool {
    for _, a := range allowed {
        if a == "*" || a == origin {
            return true
        }
    }
    return false
}
```

### 6.4 クリックジャッキング防止

```go
// SecurityHeaders ミドルウェア内で設定
w.Header().Set("X-Frame-Options", "DENY")
// CSP の frame-ancestors ディレクティブでも防止（sec 6.2 参照）
// frame-ancestors 'none' は X-Frame-Options: DENY と同等
```

| ヘッダ | 値 | 効果 |
|---|---|---|
| `X-Frame-Options` | `DENY` | 全てのフレーム内表示を拒否 |
| CSP `frame-ancestors` | `'none'` | 同上（CSP Level 2、より新しいブラウザ対応） |

Broth は両方を設定する（後方互換性のため）。

### 6.5 レート制限

#### アルゴリズム選択: Token Bucket

**決定**: Token Bucket アルゴリズムを採用する。

**根拠**:

| アルゴリズム | 特性 | 評価 |
|---|---|---|
| **Token Bucket** | バースト許容 + 平均レート制限。実装が単純 | **採用** |
| Sliding Window Log | 正確だがメモリ使用量が大きい | 不採用 |
| Sliding Window Counter | Token Bucket と同等の精度で省メモリ | 将来のオプション |
| Fixed Window | ウィンドウ境界でバーストが発生する | 不採用 |

Token Bucket は Go 標準ライブラリの `golang.org/x/time/rate` で利用可能であり、ARCHITECTURE.md の「標準ライブラリ最大活用」原則に合致する。

#### 実装

```go
// broth/middleware/rate_limit.go
package middleware

import (
    "net"
    "net/http"
    "sync"

    "golang.org/x/time/rate"
)

// RateLimiter はIPアドレスベースのレート制限を行う。
type RateLimiter struct {
    mu       sync.Mutex
    limiters map[string]*rate.Limiter
    rps      rate.Limit
    burst    int
}

// NewRateLimiter は RateLimiter を生成する。
func NewRateLimiter(rps int, burst int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        rps:      rate.Limit(rps),
        burst:    burst,
    }
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    limiter, exists := rl.limiters[ip]
    if !exists {
        limiter = rate.NewLimiter(rl.rps, rl.burst)
        rl.limiters[ip] = limiter
    }
    return limiter
}

// RateLimit はグローバルレート制限ミドルウェアを返す。
func RateLimit(cfg RateLimitConfig) func(http.Handler) http.Handler {
    limiter := NewRateLimiter(cfg.GlobalRPS, cfg.GlobalRPS*2)

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip, _, _ := net.SplitHostPort(r.RemoteAddr)
            if !limiter.getLimiter(ip).Allow() {
                w.Header().Set("Retry-After", "1")
                HandleError(w, r, http.StatusTooManyRequests,
                    errors.New("rate limit exceeded"))
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

#### ログインエンドポイントの強化レート制限

ブルートフォース攻撃を防止するため、ログインエンドポイントには追加のレート制限を適用する。

```go
// broth/middleware/rate_limit.go（追加）

// LoginRateLimit はログインエンドポイント用の厳密なレート制限。
// IP + メールアドレスの組み合わせで制限する。
func LoginRateLimit(cfg RateLimitConfig) func(http.Handler) http.Handler {
    type key struct {
        IP    string
        Email string
    }

    var (
        mu       sync.Mutex
        attempts = make(map[key][]time.Time)
    )

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip, _, _ := net.SplitHostPort(r.RemoteAddr)
            email := r.FormValue("email")

            k := key{IP: ip, Email: email}
            now := time.Now()
            windowStart := now.Add(-cfg.LoginWindow)

            mu.Lock()
            // ウィンドウ外の試行を除去
            valid := attempts[k][:0]
            for _, t := range attempts[k] {
                if t.After(windowStart) {
                    valid = append(valid, t)
                }
            }
            attempts[k] = valid

            if len(valid) >= cfg.LoginMaxAttempts {
                mu.Unlock()
                w.Header().Set("Retry-After",
                    strconv.Itoa(int(cfg.LoginWindow.Seconds())))
                HandleError(w, r, http.StatusTooManyRequests,
                    errors.New("too many login attempts, please try again later"))
                return
            }

            attempts[k] = append(attempts[k], now)
            mu.Unlock()

            next.ServeHTTP(w, r)
        })
    }
}
```

#### ルートへの適用例

```go
// modules/account/routes.go
func (h *Handler) Routes() []router.Route {
    return []router.Route{
        {Pattern: "POST /login", Handler: middleware.LoginRateLimit(cfg.Security.RateLimit)(
            http.HandlerFunc(h.Login),
        )},
    }
}
```

#### ストレージバックエンド

| バックエンド | 構成 | 特性 |
|---|---|---|
| **インメモリ（sync.Mutex + map）** | 最小構成（デフォルト） | 単一インスタンス向け。再起動でリセット |
| **データベース** | 標準構成 | 複数インスタンスで共有可能。`golang.org/x/time/rate` の状態をDBに永続化 |
| **Redis** | 拡張構成 | 高性能・複数インスタンス対応。`INCR` + `EXPIRE` で実装 |

デフォルトはインメモリ。複数インスタンス構成ではDB or Redis バックエンドを選択する。

---

## 7. 秘密情報管理

### 7.1 SECRET_KEY の管理

```go
// config/security.go（再掲・詳細）

// SecurityConfig の SecretKey フィールド
type SecurityConfig struct {
    // SecretKey は最低32バイトのランダム文字列。
    // 以下の用途に使用される:
    //   - セッション Cookie の署名
    //   - CSRF トークンの生成
    //   - JWT の HMAC 署名
    //
    // 生成方法: broth generate secret-key
    SecretKey string `env:"SECRET_KEY" required:"true"`
}
```

#### SECRET_KEY の生成

```bash
# CLI コマンドで生成
$ broth generate secret-key
Generated secret key:
  <your-generated-key-here>

Add this to your .env file:
  SECRET_KEY=<your-generated-key-here>
```

```go
// broth/cli/generate_secret.go
func generateSecretKey() string {
    b := make([]byte, 48) // 48バイト = 64文字の Base64
    if _, err := rand.Read(b); err != nil {
        panic(err)
    }
    return base64.StdEncoding.EncodeToString(b)
}
```

#### 起動時の検証

```go
// broth/config/validate.go

// ValidateSecurity は起動時にセキュリティ設定を検証する。
// 不備がある場合はパニックし、安全でない状態での起動を防止する。
func ValidateSecurity(cfg *SecurityConfig, env string) {
    // SECRET_KEY の存在チェック
    if cfg.SecretKey == "" {
        panic("broth: SECRET_KEY is required. Run 'broth generate secret-key' to generate one.")
    }

    // SECRET_KEY の長さチェック（最低32バイト）
    decoded, err := base64.StdEncoding.DecodeString(cfg.SecretKey)
    if err != nil || len(decoded) < 32 {
        panic("broth: SECRET_KEY must be at least 32 bytes. Run 'broth generate secret-key' to generate a new one.")
    }

    // 本番環境での追加チェック
    if env == "production" {
        // CSRF の Secure フラグ
        if !cfg.CSRF.Secure {
            panic("broth: CSRF_SECURE must be true in production")
        }
    }
}
```

### 7.2 環境変数ベースの設定

PROJECT_STRUCTURE.md sec 3.9 で定義された `.env` / `.env.example` パターンに準拠する。

```bash
# .env.example（git管理対象 -- 実際の値は入れない）
# セキュリティ設定
SECRET_KEY=           # 必須。broth generate secret-key で生成
# AUTH_SESSION_LIFETIME=24h
# AUTH_REMEMBER_ME_DURATION=720h
# AUTH_BEARER_LIFETIME=15m
# AUTH_REFRESH_LIFETIME=168h

# CSRF設定
# CSRF_ENABLED=true
# CSRF_SECURE=true    # 本番環境では true 必須

# CORS設定（API用）
# CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
# CORS_ALLOW_CREDENTIALS=false

# レート制限
# RATE_LIMIT_GLOBAL_RPS=100
# RATE_LIMIT_LOGIN_MAX=5
# RATE_LIMIT_LOGIN_WINDOW=15m
```

```bash
# .env（gitignore対象 -- 実際の値を入れる）
SECRET_KEY=aB3dEfGhIjKlMnOpQrStUvWxYz012345678901234567=
DATABASE_URL=postgres://user:pass@localhost:5432/myapp_dev?sslmode=disable
```

### 7.3 SECRET_KEY のローテーション

```go
// config/security.go

type SecurityConfig struct {
    SecretKey string `env:"SECRET_KEY" required:"true"`

    // PreviousSecretKey はローテーション中の旧キー。
    // 設定されている場合、新キーでの検証に失敗したら旧キーで再検証する。
    // ローテーション完了後に削除する。
    PreviousSecretKey string `env:"PREVIOUS_SECRET_KEY"`
}
```

ローテーション手順:

1. 新しい SECRET_KEY を生成する（`broth generate secret-key`）
2. 現在の SECRET_KEY を `PREVIOUS_SECRET_KEY` に移す
3. 新しい SECRET_KEY を `SECRET_KEY` に設定する
4. デプロイする（新旧両方のキーで検証が行われる）
5. 全セッションが更新されたら `PREVIOUS_SECRET_KEY` を削除する

---

## 8. セキュリティヘッダのデフォルト設定

### デフォルトで設定されるヘッダ一覧

Broth が `SecurityHeaders` ミドルウェアでデフォルトで設定するHTTPヘッダの一覧。

| ヘッダ | デフォルト値 | 目的 |
|---|---|---|
| `X-Content-Type-Options` | `nosniff` | MIME タイプスニッフィング防止 |
| `X-Frame-Options` | `DENY` | クリックジャッキング防止 |
| `X-XSS-Protection` | `0` | ブラウザXSSフィルタ無効化（CSPを使うため） |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Referer ヘッダの情報漏洩防止 |
| `Content-Security-Policy` | 後述の詳細値 | XSS・インジェクション防止 |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` | HTTPS の強制（本番のみ） |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` | ブラウザ機能の制限 |

### SecurityHeaders ミドルウェアの実装

```go
// broth/middleware/security_headers.go
package middleware

import "net/http"

// SecurityHeadersConfig はセキュリティヘッダの設定。
type SecurityHeadersConfig struct {
    // ContentSecurityPolicy は CSP ヘッダの値。空文字列の場合はデフォルト値を使用。
    ContentSecurityPolicy string
    // HSTSEnabled は HSTS ヘッダを設定するか。本番環境では true。
    HSTSEnabled bool
    // HSTSMaxAge は HSTS の max-age（秒）。デフォルト: 63072000（2年）。
    HSTSMaxAge int
}

// SecurityHeaders はセキュリティヘッダを設定するミドルウェアを返す。
func SecurityHeaders(cfgs ...SecurityHeadersConfig) func(http.Handler) http.Handler {
    cfg := SecurityHeadersConfig{
        HSTSMaxAge: 63072000,
    }
    if len(cfgs) > 0 {
        cfg = cfgs[0]
    }

    csp := cfg.ContentSecurityPolicy
    if csp == "" {
        csp = "default-src 'self'; " +
            "script-src 'self'; " +
            "style-src 'self' 'unsafe-inline'; " +
            "img-src 'self' data:; " +
            "font-src 'self'; " +
            "connect-src 'self'; " +
            "frame-ancestors 'none'; " +
            "base-uri 'self'; " +
            "form-action 'self'"
    }

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            h := w.Header()

            // 全リクエスト共通
            h.Set("X-Content-Type-Options", "nosniff")
            h.Set("X-Frame-Options", "DENY")
            h.Set("X-XSS-Protection", "0") // CSPを使うため無効化
            h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
            h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

            // コンテキストに応じた CSP
            rc := GetRequestContext(r.Context())
            if rc == ContextSSR {
                h.Set("Content-Security-Policy", csp)
            }
            // API コンテキストでは CSP を設定しない（JSON レスポンスにはCSP不要）

            // HSTS（本番環境のみ）
            if cfg.HSTSEnabled {
                h.Set("Strict-Transport-Security",
                    "max-age="+strconv.Itoa(cfg.HSTSMaxAge)+"; includeSubDomains")
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

### オーバーライド方法

```go
// config/middleware.go でカスタム設定を渡す
middleware.SecurityHeaders(middleware.SecurityHeadersConfig{
    ContentSecurityPolicy: "default-src 'self'; script-src 'self' https://cdn.example.com",
    HSTSEnabled:           cfg.App.Env == "production",
    HSTSMaxAge:            63072000,
})
```

エンドポイント単位でヘッダを上書きしたい場合は、ハンドラ内で直接設定する。ミドルウェアは `Set` を使うため、ハンドラ側の `Set` で上書き可能。

```go
// modules/embed/handler.go
func (h *Handler) Embed(w http.ResponseWriter, r *http.Request) {
    // このエンドポイントだけフレーム内表示を許可
    w.Header().Set("X-Frame-Options", "SAMEORIGIN")
    w.Header().Set("Content-Security-Policy",
        "frame-ancestors 'self' https://partner.example.com")
    // ...
}
```

---

## 9. 設計判断の記録

### ADR-SEC001: コンテキスト自動適用の導入

**状況**: SSR と API で適用すべきセキュリティスタックが異なる。既存フレームワークはルートグループで手動分離する。

**決定**: リクエスト単位でSSR/APIを自動判定し、適切なセキュリティスタックを自動適用する。

**根拠**:
- ルートグループの手動分離は設定ミスの温床であり、Secure by Default 原則に反する
- 判定ロジックは5つのHTTPヘッダの静的チェックであり、予測可能かつデバッグ可能
- デフォルトをSSR（安全側）に倒すことで、誤判定時もセキュリティが維持される
- ルート単位のオーバーライド（`ForceContext`）で明示的な制御が可能

**リスクと緩和策**:
- リスク: 開発者が自動判定を理解せず混乱する
- 緩和: 開発モードでのヘッダ出力（`X-Broth-Context`）、`broth routes` での一覧表示、ドキュメントの充実

### ADR-SEC002: CSRF に Synchronizer Token パターンを採用

**状況**: CSRFトークンのパターンとして Synchronizer Token とダブルサブミットCookieがある。

**決定**: Synchronizer Token パターンを採用する。

**根拠**:
- Broth は `broth/session` によるセッション基盤を標準搭載しており、追加コストが低い
- サブドメインからの Cookie 操作によるバイパス攻撃に対してより堅牢
- Django と同じパターンであり、Django の成熟した設計を踏襲できる

### ADR-SEC003: パスワードハッシュのデフォルトに bcrypt を採用

**状況**: パスワードハッシュアルゴリズムとして bcrypt, argon2id, scrypt がある。

**決定**: bcrypt をデフォルトとし、argon2id はオプションとして提供する。

**根拠**:
- `golang.org/x/crypto/bcrypt` はGo準標準ライブラリであり、追加依存が最小
- bcrypt は20年以上の実績があり、既知の脆弱性がない
- argon2id はメモリハード関数としてより理論的に安全だが、パラメータ調整が複雑
- ARCHITECTURE.md の「サードパーティ依存は最小限」原則に合致

### ADR-SEC004: レート制限に Token Bucket を採用

**状況**: レート制限アルゴリズムとして Token Bucket, Sliding Window, Fixed Window がある。

**決定**: Token Bucket アルゴリズムを採用する。

**根拠**:
- `golang.org/x/time/rate` でGoの準標準として提供されている
- バースト許容と平均レート制限のバランスが良い
- 実装が単純で、インメモリ / DB / Redis の各バックエンドに移植しやすい

### ADR-SEC005: セキュリティヘッダのデフォルト有効化

**状況**: セキュリティヘッダをデフォルトで有効にするか、オプトインにするか。

**決定**: デフォルトで有効にする。オプトアウトは個別のヘッダ単位で可能。

**根拠**:
- Secure by Default 原則に合致
- セキュリティヘッダを「知らなかったから設定しなかった」という事態を防止
- 個別のオーバーライドが可能なため、特殊なケースにも対応できる
- Django の `SecurityMiddleware`、Laravel のデフォルトヘッダと同じアプローチ

### ADR-SEC006: JWT に `golang-jwt/jwt` を採用

**状況**: JWT の生成・検証に外部ライブラリ（`golang-jwt/jwt` 等）を使うか、自前実装するか。

**決定**: `github.com/golang-jwt/jwt/v5` を採用する。

**根拠**:
- JWT の自前実装はタイミング攻撃対策、アルゴリズム混同攻撃（alg=none 問題）、ヘッダインジェクション等のセキュリティ上のエッジケースが多く、「約100行」では安全な実装が困難
- `golang-jwt/jwt` は Go エコシステムにおける事実上の標準 JWT ライブラリであり、広くセキュリティレビューを受けている
- ARCHITECTURE.md の「サードパーティ依存は最小限」原則は維持しつつ、セキュリティに関わる実績あるライブラリは例外として許容する
- `golang-jwt/jwt` の API は明快で、Go イディオムに沿った設計であり P1 原則と矛盾しない
- RSA/ECDSA への将来的な拡張も、ライブラリが標準でサポートしている

**変更履歴**: 初期設計では HMAC-SHA256 自前実装としていたが、セキュリティリスクの懸念から `golang-jwt/jwt` 採用に変更

---

## 付録A: セキュリティチェックリスト

新しいモジュールやエンドポイントを追加する際のセキュリティチェックリスト。

- [ ] 認証が必要なエンドポイントに `RequireAuth` ミドルウェアを適用した
- [ ] リソースの操作に認可チェック（Policy）を実装した
- [ ] フォームに `{{csrfField .Request}}` を含めた
- [ ] ユーザー入力をそのまま `template.HTML` にキャストしていない
- [ ] SQLクエリにプレースホルダ（`$1`, `$2`）を使用している（SQLインジェクション防止）
- [ ] パスワードを平文で保存していない（`auth.DefaultPasswordHasher` を使用）
- [ ] 秘密情報を `.env` に配置し、ソースコードにハードコードしていない
- [ ] APIエンドポイントのレート制限を検討した
- [ ] エラーメッセージにスタックトレースやDB情報が含まれていない

## 付録B: PROJECT_STRUCTURE.md との整合マッピング

| 本設計書の要素 | PROJECT_STRUCTURE.md の対応箇所 |
|---|---|
| `config/security.go` | sec 3.2 `config/` -- アプリケーション設定（新規追加ファイル） |
| `config/middleware.go` のセキュリティMW | sec 3.2 `config/middleware.go` -- ミドルウェア設定 |
| `.env` の SECRET_KEY 等 | sec 3.9 `.env` -- 環境変数定義 |
| `.env.example` のセキュリティ設定テンプレート | sec 3.9 `.env.example` -- 環境変数テンプレート |
| `broth/auth` パッケージ | ARCHITECTURE.md sec 4 `broth/auth` -- 認証・認可 |
| `broth/session` パッケージ | ARCHITECTURE.md sec 4 `broth/session` -- セッション管理 |
| `broth/middleware` のセキュリティMW | ARCHITECTURE.md sec 4 `broth/middleware` -- 標準ミドルウェア群 |
| テンプレートの `{{csrfField}}` | sec 3.6 `templates/` -- 共通テンプレート |

## 付録C: broth new で生成されるセキュリティ関連ファイル

`broth new myapp` で以下のセキュリティ関連ファイルが生成される。

| パス | 内容 |
|---|---|
| `config/security.go` | SecurityConfig 構造体の定義 |
| `config/middleware.go` | GlobalMiddleware にセキュリティMWを含む |
| `.env.example` | SECRET_KEY 等のセキュリティ設定テンプレート |
| `templates/layouts/base.html` | CSRFメタタグ・csrfField の使用例 |
| `static/js/app.js` | AJAX用CSRFトークン自動付与の実装 |
