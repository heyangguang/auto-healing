<p align="center">
  <img src="docs/images/pangolin-logo-full.png" alt="Pangolin - 自動修復プラットフォーム" width="400" />
</p>

<h1 align="center">Pangolin — Auto-Healing Platform</h1>

<p align="center">
  <strong>エンタープライズグレードのインテリジェントIT運用自己修復プラットフォーム</strong>
</p>

<p align="center">
  <a href="https://github.com/heyangguang/auto-healing/releases"><img src="https://img.shields.io/github/v/release/heyangguang/auto-healing?style=flat-square&color=blue" alt="Release" /></a>
  <a href="https://github.com/heyangguang/auto-healing/blob/main/LICENSE"><img src="https://img.shields.io/github/license/heyangguang/auto-healing?style=flat-square" alt="License" /></a>
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go Version" />
  <img src="https://img.shields.io/badge/PostgreSQL-15+-336791?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL" />
  <img src="https://img.shields.io/badge/Ansible-2.14+-EE0000?style=flat-square&logo=ansible&logoColor=white" alt="Ansible" />
  <img src="https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react&logoColor=white" alt="React" />
</p>

<p align="center">
  <a href="#-クイックスタート">クイックスタート</a> •
  <a href="#-主要機能">主要機能</a> •
  <a href="#-アーキテクチャ">アーキテクチャ</a> •
  <a href="#-デプロイ">デプロイ</a> •
  <a href="#-ドキュメント">ドキュメント</a>
</p>

<p align="center">
  <a href="./README.md">English</a> | <a href="./README_zh-CN.md">简体中文</a> | <a href="./README_ja.md">日本語</a>
</p>

---

## 🌟 Auto-Healing とは？

**Auto-Healing Platform（AHS）** は、オープンソースのエンタープライズグレードIT運用自動化・自己修復プラットフォームです。ITSMチケット、CMDB資産、Ansible Playbook、承認ワークフローを統合し、**「問題検知 → インテリジェントマッチング → 自動修復 → 完全監査追跡」** のシームレスな運用クローズドループを実現します。

> **問題の発見から解決までを自動化 — 完全な監査証跡付き。**

```
┌──────────────┐     ┌────────────────────┐     ┌─────────────────────┐     ┌──────────────────┐
│  外部 ITSM    │────▶│  アラート取込・解析  │────▶│  スマートルール     │────▶│  自動修復 + 承認   │
│  監視システム  │     │  (Plugin 統合)      │     │  エンジン           │     │  (DAG ワークフロー)│
└──────────────┘     └────────────────────┘     └─────────────────────┘     └──────────────────┘
                                                                                     │
                                ◀────────── 完全監査ログ & リアルタイム SSE ──────────┘
```

### 💡 なぜ Auto-Healing を選ぶのか？

| 課題 | 従来の方法 | Auto-Healing |
|------|-----------|-------------|
| **アラート疲れ** | 毎日数千のアラートを手動対応 | スマートルールマッチングで自動解決 |
| **遅い MTTR** | 平均応答時間 30 分以上 | 自動修復で **2 分未満** |
| **繰り返し作業** | ディスク整理やサービス再起動を手動実行 | DAG ワークフローで完全自動化 |
| **監査証跡なし** | 運用は属人的知識に依存 | 改ざん不可能なフォレンジックログ |
| **ツールサイロ** | ITSM、CMDB、スクリプトが分断 | 統合プラットフォーム・プラグイン連携 |

---

## ✨ 主要機能

### 🔄 自己修復エンジン
- **ビジュアル DAG ワークフローエディタ** — ドラッグ＆ドロップで複雑な修復フローを構築
- **9 種類のノードタイプ** — ホスト抽出、CMDB 検証、条件分岐、実行、承認、通知、変数設定、ループ、計算
- **デュアルトリガーモード** — 自動（ゼロタッチ）またはマニュアル（承認ゲート）
- **Dry-Run サンドボックス** — 副作用なしでワークフロー実行をシミュレーション
- **SSE リアルタイムストリーミング** — ノード状態更新が 200ms 未満

### 🔌 プラグイン統合
- **ユニバーサル ITSM/CMDB アダプタ** — ServiceNow、Jira、Zabbix、カスタムシステム対応
- **フィールドマッピングエンジン** — 外部から内部へのフィールドマッピングをビジュアル設定
- **スマートフィルタリング** — AND/OR ロジックグループで選択的データ同期
- **双方向ライトバック** — 修復後にチケットステータスを自動更新

### ⚡ 実行センター
- **3 つのトリガーモード** — 手動ランチパッド、スケジュール（Cron）、修復フロートリガー
- **Ansible エンジン** — Docker / Local デュアルモード実行
- **3 状態検証** — ホスト到達不能による偽の成功を防止
- **ランタイムパラメータオーバーライド** — 実行ごとのホストと変数の動的注入

### 🔐 セキュリティ
- **RBAC** — リソースレベル + オペレーションレベルの権限制御
- **JWT + SAML 2.0** — エンタープライズ SSO（ADFS、Azure AD）対応
- **JIT プロビジョニング** — 初回 SSO ログイン時にアカウント自動作成
- **シークレット管理** — SSH、API キー、トークンの集中管理
- **監査ログ** — すべての操作をオペレーター帰属情報付きで記録

---

## 🏗 アーキテクチャ

```
┌─────────────────────────────────────────────────────────────────────┐
│                     フロントエンド (React SPA)                        │
│   React 19 · Umi 4 · Ant Design 6 · ProComponents · React Flow     │
└──────────────────────────┬──────────────────────────────────────────┘
                           │ REST API
┌──────────────────────────▼──────────────────────────────────────────┐
│                       バックエンド (Go)                              │
│   Gin HTTP Router · レイヤードアーキテクチャ                          │
│   Handler → Service → Repository → Database                        │
│                                                                     │
│   ┌─────────────────┐ ┌───────────────┐ ┌────────────────────────┐ │
│   │ 修復エンジン     │ │ プラグイン     │ │ 実行エンジン            │ │
│   │ (DAG Executor)  │ │ (ITSM/CMDB)   │ │ (Ansible Runner)      │ │
│   └─────────────────┘ └───────────────┘ └────────────────────────┘ │
│   ┌─────────────────┐ ┌───────────────┐ ┌────────────────────────┐ │
│   │ 認証 (JWT+SAML) │ │ 通知エンジン   │ │ バックグラウンドスケジューラ│ │
│   └─────────────────┘ └───────────────┘ └────────────────────────┘ │
└──────────────────────────┬──────────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────────┐
│                         データ層                                    │
│   PostgreSQL (JSONB · UUID · TIMESTAMPTZ) · Redis                   │
└──────────────────────────┬──────────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────────┐
│                         実行層                                      │
│   Ansible Engine (Local / Docker デュアルモード)                      │
│   SSH クレデンシャル注入 · Jinja2 変数レンダリング                     │
└─────────────────────────────────────────────────────────────────────┘
```

### テクノロジースタック

| レイヤー | 技術 | 用途 |
|---------|------|------|
| **言語** | Go 1.24+ | 高性能・低メモリバックエンド |
| **Web** | Gin | 業界最速の Go HTTP フレームワーク |
| **ORM** | GORM | JSONB サポート付き充実した ORM |
| **DB** | PostgreSQL 15+ | JSONB・UUID・TIMESTAMPTZ ネイティブ対応 |
| **キャッシュ** | Redis 7+ | キャッシュとメッセージキュー |
| **認証** | JWT + SAML 2.0 | エンタープライズ SSO 対応 |
| **リアルタイム** | SSE | 軽量な単方向ストリーミング |
| **式エンジン** | expr-lang/expr | 高性能 Go 式評価 |
| **自動化** | Ansible 2.14+ | インフラ自動化エンジン |
| **フロントエンド** | React 19 + Umi 4 | エンタープライズ React フレームワーク |
| **UI** | Ant Design 6 | エンタープライズ UI エコシステム |
| **ワークフロー** | React Flow | プロフェッショナル DAG エディタ |

---

## 🚀 クイックスタート

### 前提条件

| 依存関係 | バージョン | 必須 |
|---------|----------|------|
| Go | 1.24+ | ✅ |
| PostgreSQL | 15+ | ✅ |
| Redis | 7+ | ✅ |
| Ansible | 2.14+ | ✅ |
| Docker | 24+ | オプション |

### インストール

```bash
# リポジトリをクローン
git clone https://github.com/heyangguang/auto-healing.git
cd auto-healing

# インフラを起動（PostgreSQL + Redis）
cd deployments/docker
docker compose up -d
cd ../..

# ビルド
go build -o bin/server ./cmd/server
go build -o bin/init-admin ./cmd/init-admin

# 管理者アカウントの初期化
./bin/init-admin

# サーバー起動
./bin/server
```

### 動作確認

```bash
# ヘルスチェック
curl http://localhost:8080/health

# ログイン
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}'
```

> **デフォルト認証情報:** `admin` / `admin123456`  
> ⚠️ **本番環境では必ずパスワードを変更してください！**

---

## 📦 ビルド済みバイナリ

[Releases ページ](https://github.com/heyangguang/auto-healing/releases)からダウンロードできます。

### サポートプラットフォーム

| OS | アーキテクチャ | バイナリ名 |
|----|-------------|-----------|
| **Linux** | x86_64 (amd64) | `auto-healing-linux-amd64` |
| **Linux** | ARM64 | `auto-healing-linux-arm64` |
| **macOS** | Intel (amd64) | `auto-healing-darwin-amd64` |
| **macOS** | Apple Silicon (arm64) | `auto-healing-darwin-arm64` |
| **Windows** | x86_64 (amd64) | `auto-healing-windows-amd64.exe` |
| **Windows** | ARM64 | `auto-healing-windows-arm64.exe` |

### ソースからビルド

```bash
# 全プラットフォーム向けクロスコンパイル
GOOS=linux   GOARCH=amd64 go build -o bin/auto-healing-linux-amd64      ./cmd/server
GOOS=linux   GOARCH=arm64 go build -o bin/auto-healing-linux-arm64      ./cmd/server
GOOS=darwin  GOARCH=amd64 go build -o bin/auto-healing-darwin-amd64     ./cmd/server
GOOS=darwin  GOARCH=arm64 go build -o bin/auto-healing-darwin-arm64     ./cmd/server
GOOS=windows GOARCH=amd64 go build -o bin/auto-healing-windows-amd64.exe ./cmd/server
GOOS=windows GOARCH=arm64 go build -o bin/auto-healing-windows-arm64.exe ./cmd/server
```

---

## 🐳 Docker イメージ

公式 Docker イメージは GitHub Container Registry で公開しています：

| イメージ | 説明 |
|---------|------|
| `ghcr.io/heyangguang/auto-healing` | **サーバー** — メイン API + 修復エンジン |
| `ghcr.io/heyangguang/auto-healing-executor` | **エグゼキューター** — 隔離された Ansible 実行環境 |

### Docker クイックスタート

```bash
# イメージのプル
docker pull ghcr.io/heyangguang/auto-healing:latest
docker pull ghcr.io/heyangguang/auto-healing-executor:latest

# サーバー起動（PostgreSQL と Redis が必要）
docker run -d --name auto-healing \
  -p 8080:8080 \
  -e AH_DATABASE_HOST=your-postgres-host \
  -e AH_REDIS_HOST=your-redis-host \
  ghcr.io/heyangguang/auto-healing:latest
```

### エグゼキューターとは？

プラットフォームは Ansible Playbook の実行に **2つのモード** を提供します：

| モード | 動作方式 | 推奨用途 |
|--------|---------|---------|
| **Local** | サーバーホスト上で直接 Ansible を実行 | 開発、シンプルな構成 |
| **Docker** | `auto-healing-executor` コンテナ内で実行 | 本番環境（隔離・再現性） |

> 💡 Docker モードはクリーンな隔離環境で実行するため、依存関係の競合を防ぎ安全性を向上させます。

---

## 🚢 デプロイ

### システム要件

| 規模 | CPU | メモリ | ディスク |
|------|-----|--------|---------|
| **小規模** (< 50 ホスト) | 2 コア | 2 GB | 20 GB |
| **中規模** (50-500 ホスト) | 4 コア | 4 GB | 50 GB |
| **大規模** (500+ ホスト) | 8+ コア | 8+ GB | 100+ GB |

### 本番環境チェックリスト

- [ ] デフォルトの管理者パスワードを変更
- [ ] 強力な JWT シークレットを設定
- [ ] PostgreSQL SSL を有効化
- [ ] ログローテーションを設定
- [ ] データベースバックアップを設定
- [ ] リバースプロキシ（Nginx/Caddy）を TLS 付きで設定
- [ ] Redis 認証を有効化

---

## 📖 ドキュメント

| ドキュメント | 説明 |
|------------|------|
| [API リファレンス](docs/openapi.yaml) | OpenAPI 3.0 仕様 |
| [API テストガイド](docs/api-testing-guide.md) | cURL サンプルとテストワークフロー |
| [プロジェクト紹介](docs/auto_healing_project_intro.md) | 製品の包括的な概要 |

---

## 🤝 コントリビューション

コントリビューションを歓迎します！PR を提出する前にコントリビューションガイドラインをお読みください。

```bash
git clone https://github.com/heyangguang/auto-healing.git
cd auto-healing
go mod tidy
cd deployments/docker && docker compose up -d && cd ../..
go run ./cmd/server
```

---

## 📄 ライセンス

このプロジェクトは [Apache License 2.0](LICENSE) の下でライセンスされています。

---

<p align="center">
  <strong>⭐ このプロジェクトが役立つと思ったら、スターをお願いします！</strong>
</p>

<p align="center">
  Made with ❤️ by <a href="https://github.com/heyangguang">Auto-Healing Team</a>
</p>
