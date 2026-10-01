# MYSTANDARD — Standar Proyek Open Source

> Acuan tunggal untuk semua proyek open source: **penulisan README, tampilan (layout sidebar & menu), font, color scheme light/dark, komponen UI, fitur dasar, dan login**.
> Setiap proyek baru mengikuti dokumen ini apa adanya, sehingga semua proyek terlihat dan bekerja dengan cara yang sama.

**Bagian A — Standar**

1. [Cara Pakai](#1-cara-pakai)
2. [Prinsip Dasar](#2-prinsip-dasar)
3. [Struktur Proyek](#3-struktur-proyek)
4. [Font](#4-font)
5. [Color Scheme (Light & Dark)](#5-color-scheme-light--dark)
6. [Ukuran, Radius, Shadow, Breakpoint](#6-ukuran-radius-shadow-breakpoint)
7. [Layout Sidebar](#7-layout-sidebar)
8. [Layout Menu](#8-layout-menu)
9. [Komponen UI](#9-komponen-ui)
10. [Ikon](#10-ikon)
11. [Fitur Dasar](#11-fitur-dasar)
12. [Login & Autentikasi](#12-login--autentikasi)
13. [Penulisan README.md](#13-penulisan-readmemd)
14. [CLAUDE.md](#14-claudemd)
15. [Checklist Proyek Baru](#15-checklist-proyek-baru)

**Bagian B — Lampiran Kode Starter**

- [A. main.css](#lampiran-a-maincss)
- [B. index.html & auth.html](#lampiran-b-indexhtml--authhtml)
- [C. JavaScript inti](#lampiran-c-javascript-inti)
- [D. auth-page.js](#lampiran-d-auth-pagejs)

---

# Bagian A — Standar

## 1. Cara Pakai

### 1.1 Memulai proyek baru

1. Buat struktur folder sesuai [§3](#3-struktur-proyek).
2. Salin kode starter dari [Bagian B](#bagian-b--lampiran-kode-starter) ke path yang tertulis di atas setiap blok.
3. Unduh font ([§4.1](#41-file-dan-lisensi)) dan buat ikon aplikasi ([§10.2](#102-ikon-aplikasi--gaya-macos)).
4. Ganti nama sesuai [§1.2](#12-daftar-ganti-nama).
5. Susun menu di array `NAV` (`web/static/js/app.js`, [§8](#8-layout-menu)).
6. Tambah halaman dari template view ([Lampiran C](#lampiran-c-javascript-inti)).
7. Implementasikan backend auth sesuai [§12](#12-login--autentikasi).
8. Tulis README dengan template [§13](#13-penulisan-readmemd).
9. Jalankan [§15 Checklist](#15-checklist-proyek-baru).

### 1.2 Daftar ganti nama

Kode starter memakai nama `App`. Ganti semuanya dengan nama proyek (contoh: `Nimbus`).

| Di starter | Di proyek (contoh) |
|---|---|
| `App` (teks UI, judul halaman, konstanta `APP`) | `Nimbus` |
| `"App Mono"` (font-family) | `"Nimbus Mono"` |
| `app_theme`, `app_sidebar`, `app_groups` (localStorage) | `nimbus_theme`, `nimbus_sidebar`, `nimbus_groups` |
| `app_session` (cookie sesi) | `nimbus_session` |
| `APP_*` (environment variable) | `NIMBUS_*` |
| Issuer TOTP `App` | `Nimbus` |
| `/data/app/auth.json` | `/data/nimbus/auth.json` |

Cek sisa nama starter:

```bash
grep -rnE '\bApp\b|app_|APP_|"App Mono"' web/ internal/ cmd/
```

### 1.3 Prompt untuk AI agent

```text
Proyek ini mengikuti MYSTANDARD.md. Saat membuat atau mengubah UI, login, fitur dasar, atau README:
1. Pakai token warna (light & dark), font, ukuran, dan komponen dari §4–§10 dan Lampiran A; jangan membuat warna atau ukuran baru.
2. Menu sidebar hanya lewat array NAV (§8). Halaman baru memakai template view (Lampiran C).
3. Login/auth mengikuti §12 persis (alur, parameter keamanan, pesan). Jangan melemahkan aturan apa pun.
4. README mengikuti struktur dan gaya §13 (Bahasa Indonesia).
5. Teks UI Bahasa Inggris; kode, komentar, commit Bahasa Inggris (Conventional Commits).
6. Jalankan checklist §15 dan laporkan item yang belum terpenuhi.
```

---

## 2. Prinsip Dasar

| Area | Standar |
|---|---|
| Distribusi | **Satu binary Go** (`CGO_ENABLED=0`). Frontend di-embed dengan `go:embed`. |
| Frontend | HTML + CSS + **vanilla ES modules**. Tanpa npm, bundler, framework, atau CDN. Aset eksternal nol. |
| Halaman HTML | Disajikan `Cache-Control: no-cache` agar rilis baru langsung terpakai. |
| Router | Hash router `#/<view>?<params>`. State filter disimpan di URL agar bisa di-bookmark. |
| Tema | **Light dan dark** wajib, satu warna aksen, neutral bernuansa *stone*. |
| Font | Satu font monospace untuk **seluruh** UI: JetBrains Mono Nerd Font Mono. |
| Ikon | Lucide (ISC), di-inline sebagai SVG `<symbol>`. Ikon aplikasi bergaya macOS. |
| Bahasa | Teks UI: **Inggris**. README, `docs/`, dokumen produk: **Indonesia**. Kode, komentar, commit: **Inggris**. |
| Commit | Conventional Commits: `feat(x):`, `fix(deploy):`, `refactor(naming):`. |
| Konfigurasi | **Env-only** (`APP_*`). File contoh konfigurasi hanya dokumentasi. |
| Backend | `log/slog`, `net/http` mux standar dengan method pattern Go 1.22+ (`"GET /api/v1/items"`). Dependensi seminimal mungkin. |
| API | Prefix `/api/v1`. Error selalu JSON `{"error": "pesan"}`. `GET /healthz` publik. |
| Auth | **Aktif secara default** (password + TOTP). Bisa dimatikan lewat env untuk jaringan tertutup. |
| Komentar | Menjelaskan *kenapa*, singkat. |

---

## 3. Struktur Proyek

```text
<app>/
├── cmd/<app>/main.go          # entrypoint
├── internal/
│   ├── auth/                  # users, roles, bcrypt, TOTP, sesi, lockout, recovery
│   ├── config/                # struct + DefaultConfig() + LoadFromEnv()
│   ├── version/               # Version/Commit via -ldflags -X
│   ├── handler/               # REST routes: auth.go, users.go, <domain>.go
│   └── <domain>/              # logika aplikasi
├── web/
│   ├── embed.go               # go:embed static/*
│   └── static/
│       ├── index.html         # shell dashboard
│       ├── auth.html          # /login dan /setup
│       ├── appicon.png        # ikon aplikasi 512px (sidebar, About, login)
│       ├── favicon.png        # 64px
│       ├── fonts/             # JetBrainsMonoNerdFontMono-{Regular,Bold}.woff2 + OFL.txt
│       ├── styles/main.css
│       └── js/
│           ├── app.js         # router, sidebar (NAV), topbar, palette items
│           ├── api.js         # fetch wrapper /api/v1
│           ├── state.js       # store, events, router, localStorage
│           ├── ui.js          # h(), icon(), toast, dialog, menu, password helpers
│           ├── palette.js     # Ctrl+K
│           ├── auth-page.js   # setup, login, recovery
│           └── views/         # satu file per halaman: dashboard, profile, users, about, ...
├── build/appicon.svg          # sumber ikon aplikasi (gaya macOS)
├── deploy/                    # manifest deployment (opsional), dinomori sesuai urutan apply
├── docs/                      # panduan tambahan (Bahasa Indonesia)
├── scripts/                   # skrip bantu
├── Dockerfile                 # multi-stage: golang:<ver>-alpine → alpine
├── Makefile                   # build, test, run, docker
├── CLAUDE.md
├── LICENSE
└── README.md
```

Aturan:

- Path statis publik (`/js/`, `/styles/`, `/fonts/`, `/favicon.png`, `/appicon.png`) didaftarkan di satu fungsi allowlist (`isAsset()`). Path publik baru **wajib** ditambahkan di sana; kalau tidak, request-nya di-redirect ke login.
- Frontend ikut ter-compile ke binary: **build ulang setelah mengubah file di `web/static`**.

---

## 4. Font

### 4.1 File dan lisensi

| Item | Nilai |
|---|---|
| Font | JetBrains Mono **Nerd Font Mono** (glyph Nerd Font ikut tampil) |
| File | `web/static/fonts/JetBrainsMonoNerdFontMono-Regular.woff2`, `JetBrainsMonoNerdFontMono-Bold.woff2` |
| Lisensi | SIL OFL 1.1 — sertakan `web/static/fonts/OFL.txt` |
| Family name di CSS | `"<App> Mono"` |

Cara mendapatkan:

```bash
# 1. Unduh JetBrainsMono.zip dari https://github.com/ryanoasis/nerd-fonts/releases (aset "JetBrainsMono.zip")
# 2. Ambil dua file TTF lalu ubah ke woff2 (paket "woff2")
unzip JetBrainsMono.zip JetBrainsMonoNerdFontMono-Regular.ttf JetBrainsMonoNerdFontMono-Bold.ttf OFL.txt
woff2_compress JetBrainsMonoNerdFontMono-Regular.ttf
woff2_compress JetBrainsMonoNerdFontMono-Bold.ttf
mkdir -p web/static/fonts && mv JetBrainsMonoNerdFontMono-*.woff2 OFL.txt web/static/fonts/
```

### 4.2 Pemuatan

- `@font-face` dua berat: Regular melayani `font-weight: 100 500`, Bold melayani `600 900`. Jadi `600` (semibold) dan `700` (bold) sama-sama memakai file Bold.
- `font-display: swap`.
- Preload Regular di setiap HTML:

```html
<link rel="preload" href="/fonts/JetBrainsMonoNerdFontMono-Regular.woff2" as="font" type="font/woff2" crossorigin>
```

- Fallback stack:

```css
--font: "App Mono", "JetBrainsMono Nerd Font Mono", "JetBrains Mono", "SF Mono", "Cascadia Code", ui-monospace, Menlo, Consolas, monospace;
```

### 4.3 Skala tipografi

Base: `body { font: 12.5px/1.5 var(--font); -webkit-font-smoothing: antialiased; }`

| Pemakaian | Ukuran | Berat / catatan |
|---|---|---|
| Stat value (angka KPI) | 26px (stat tile: 24px) | 700, `letter-spacing: -.02em`, `line-height: 1.15` |
| Judul halaman detail `h1` | 24px | |
| Judul halaman `h1` | 22px | `letter-spacing: -.02em` |
| Judul login `h1` | 20px | |
| Brand di halaman login | 18px | 700, warna `--accent-text` |
| Brand di sidebar | 15px | 700, warna `--accent-text`, `letter-spacing: -.01em` |
| Judul dialog `h3` | 15px | |
| Judul card `h2`, input palette | 14px | |
| Body | 12.5px | 400 |
| Tabel, isi teks panjang | 12px | |
| Teks sekunder: subjudul card, label field, `btn-sm`, note | 11.5px | |
| Kecil: badge, chip, count sidebar, header tabel, judul grup sidebar | 11px | Judul grup: 600, `letter-spacing: .04em` |
| Mikro: `kbd`, versi di sidebar, label grup palette | 10.5px | Label grup palette: 600, uppercase, `letter-spacing: .06em` |

Aturan:

- Angka di tabel, jam, dan statistik: `font-variant-numeric: tabular-nums`.
- Hanya tiga berat: 400, 600, 700.
- Tidak ada font kedua (tidak ada sans-serif untuk heading).

---

## 5. Color Scheme (Light & Dark)

Satu aksen **burnt orange**, neutral **stone**. Semua warna lewat CSS custom property; tidak ada hex langsung di komponen. Setiap token **wajib** punya nilai light dan dark.

### 5.1 Token inti

| Token | Light | Dark | Peran |
|---|---|---|---|
| `--bg` | `#f4f3f1` | `#121110` | Latar halaman (di belakang sidebar dan panel utama) |
| `--panel` | `#ffffff` | `#1c1a19` | Panel utama, card, dialog, input |
| `--panel-2` | `#fafaf9` | `#211f1d` | Permukaan sekunder: header tabel, footer palette, blok QR |
| `--hover` | `#f1efec` | `#292624` | Hover, track meter, latar tabs/period |
| `--border` | `#e7e4e0` | `#302d2a` | Garis standar |
| `--border-strong` | `#d5d1cc` | `#423e3a` | Border hover, OTP box, avatar |
| `--text` | `#1c1917` | `#ece9e6` | Teks utama |
| `--text-2` | `#4a4542` | `#c4beb8` | Teks sekunder, link sidebar |
| `--muted` | `#7c756f` | `#948d86` | Label, keterangan, placeholder |
| `--accent` | `#c2410c` | `#ea6a2a` | Tombol primary, ikon sidebar, focus border, meter |
| `--accent-soft` | `#fcece2` | `rgb(234 106 42 / .15)` | Latar link aktif, focus ring, avatar, item palette terpilih |
| `--accent-text` | `#b13a0a` | `#f28b52` | Teks di atas `--accent-soft`, nama brand, link-btn |
| `--danger` | `#c42b1c` | `#e5584e` | Aksi destruktif, error |
| `--ok` | `#15803d` | `#4ade80` | Sukses, status sehat |
| `--shadow` | `0 1px 2px rgb(41 37 36 / .06)` | `0 1px 2px rgb(0 0 0 / .3)` | Card, panel, tab aktif |
| `--shadow-pop` | `0 8px 28px rgb(41 37 36 / .14)` | `0 10px 30px rgb(0 0 0 / .45)` | Menu, dialog, toast, palette, kartu login |

### 5.2 Warna severity dan grafik (opsional)

Untuk status bertingkat (debug → fatal), notifikasi, dan seri grafik.

| Token | Light (mark / teks) | Dark (mark / teks) |
|---|---|---|
| `--lv-debug` / `--lvt-debug` | `#2f9e83` / `#1f7a64` | `#319a80` / `#4fc0a2` |
| `--lv-info` / `--lvt-info` | `#4f7fd9` / `#2f62c0` | `#5282d8` / `#82a8ee` |
| `--lv-warn` / `--lvt-warn` | `#c98a0c` / `#94620a` | `#b88c14` / `#e0b240` |
| `--lv-error` / `--lvt-error` | `#d63d2b` / `#b72f1f` | `#d9475a` / `#f07a86` |
| `--lv-fatal` / `--lvt-fatal` | `#a3174f` / `#a3174f` | `#c0487e` / `#e57fae` |
| `--spark` / `--grid` | `#b5aea7` / `#eeebe8` | `#5e5853` / `#2a2826` |

- `--lv-*` hanya untuk **mark** (batang, titik, garis). Teks memakai `--lvt-*`.
- Tag status: teks `--lvt-*`, latar `color-mix(in srgb, var(--lvc) 14%, transparent)`.

Warna terminal ANSI (hanya jika aplikasi menampilkan output terminal berwarna):

| Token | Light | Dark |
|---|---|---|
| `--ansi-0` … `--ansi-7` | `#57534e` `#c42b1c` `#15803d` `#a16207` `#1d4ed8` `#a21caf` `#0e7490` `#78716c` | `#8f8a84` `#f07a86` `#6fd49a` `#e0b240` `#82a8ee` `#e57fae` `#5fc9d6` `#d7d3cf` |
| `--ansi-8` … `--ansi-15` | `#78716c` `#dc2626` `#16a34a` `#b45309` `#2563eb` `#c026d3` `#0891b2` `#44403c` | `#8f8a84` `#ff9a9a` `#8ee6ad` `#f5d06a` `#a5c3f5` `#f0a8d0` `#8ee0ea` `#f5f5f4` |

### 5.3 Aturan pemakaian

- **Aksen hanya untuk:** tombol primary, link sidebar aktif, ikon sidebar, focus ring, nama brand, meter, avatar, badge `role-admin`, item palette terpilih, chip aktif.
- Teks di atas `--accent` / `--danger` solid: `#fff`.
- Varian hover/tint dibuat dengan `color-mix`, bukan warna baru:
  - primary hover: `color-mix(in srgb, var(--accent) 88%, #000)`
  - border danger: `color-mix(in srgb, var(--danger) 35%, var(--border))`
  - latar badge ok: `color-mix(in srgb, var(--ok) 15%, transparent)`
- Focus: `border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft);`
- Highlight pencarian (`mark`): `color-mix(in srgb, var(--accent) 28%, transparent)`.
- Overlay dialog: `rgb(12 10 9 / .45)`.

### 5.4 Kontras (WCAG, dihitung dari token)

| Pasangan | Light | Dark |
|---|---|---|
| `--text` di `--panel` | 17.49 | 14.34 |
| `--text-2` di `--panel` | 9.45 | 9.42 |
| `--muted` di `--panel` | 4.53 | 5.30 |
| `--muted` di `--bg` | 4.09 | 5.76 |
| `--accent-text` di `--panel` | 6.04 | 7.09 |
| `#fff` di `--accent` (tombol primary) | 5.18 | **3.18** |
| `--danger` di `--panel` | 5.66 | 4.82 |
| `--ok` di `--panel` | 5.02 | 9.95 |

Catatan: di dark mode, teks putih pada tombol primary (3.18) di bawah 4.5:1 untuk teks normal. Untuk memenuhi WCAG AA, gelapkan `--accent` dark atau pakai teks gelap di tombol primary. Hindari `--muted` langsung di atas `--bg` light (4.09) untuk teks penting.

### 5.5 Mengganti aksen (identitas per proyek)

Ubah **hanya** `--accent`, `--accent-soft`, `--accent-text` di light dan dark. Syarat: `--accent-text` di `--panel` ≥ 4.5, dan `#fff` di `--accent` ≥ 4.5. Neutral, danger, ok, dan warna severity tetap.

### 5.6 Mekanisme light / dark mode

| Aspek | Standar |
|---|---|
| Penanda | Atribut `data-theme="light"` / `"dark"` pada `<html>`. Token light di `:root`, token dark di `[data-theme="dark"]`, masing-masing dengan `color-scheme` |
| Default | Mengikuti sistem (`prefers-color-scheme`) bila user belum memilih |
| Pilihan user | Disimpan di `localStorage.<app>_theme` |
| Tanpa kedip | Script inline di `<head>`, sebelum stylesheet, memasang `data-theme` |
| Tombol | Di topbar dan pojok kanan atas halaman login: ikon `moon` saat light, `sun` saat dark (kelas `.only-light` / `.only-dark`) |
| Palette | Aksi *Toggle dark mode* di command palette |
| Gambar | Latar QR code tetap putih di kedua tema |

```html
<script>
  try {
    document.documentElement.dataset.theme = localStorage.getItem('app_theme') ||
      (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
  } catch (e) {}
</script>
```

---

## 6. Ukuran, Radius, Shadow, Breakpoint

### 6.1 Radius

| Elemen | Radius |
|---|---|
| Card (`--radius`) | 10px |
| Kontrol: button, input, select, chip (`--radius-sm`) | 6px |
| Panel utama, dialog, palette | 12px |
| Kartu login | 14px |
| Link sidebar, icon button, menu, toast, search | 8px |
| Tabs (luar / dalam) | 9px / 7px |
| Badge | 5px |
| Kbd, tag kecil | 4px |
| Avatar, dot status | 50% |
| Ikon brand di sidebar / login / About | 8px / 9px / 14px |

### 6.2 Tinggi kontrol

| Kontrol | Tinggi |
|---|---|
| `.btn`, `.input`, `.select`, `.chip`, `.tab`, `.tool-search` | 28px |
| `.btn-sm`, `.icon-btn.sm` | 26px |
| `.icon-btn`, `.avatar`, `.search`, `.side-link`, item palette | 32px |
| Input di form-grid (Profile, Users) | 32px |
| Input di `.field` dialog | 34px |
| Input di halaman login | 36px |
| Tombol blok login (`.btn-block`) | 38px |
| Kotak OTP | 44 × 50px (mobile 38 × 46px) |
| Topbar | 54px |
| Input palette | 46px |

### 6.3 Spasi

| Tempat | Nilai |
|---|---|
| Padding halaman `.page` | `22px 26px 30px` (≤860px: `16px`) |
| Jarak `page-head` → konten | 18px |
| Gap grid card | 14px; antar baris grid `.mt` = 14px |
| Padding card | `16px 18px` |
| Padding dialog | 20px |
| Padding kartu login | `28px 30px 30px` |
| Sidebar | `14px 10px 10px`; jarak antar grup 12px |

### 6.4 Breakpoint

| Lebar | Perubahan |
|---|---|
| ≤ 1100px | `grid-4` → 2 kolom; `grid-2`, `grid-2e` → 1 kolom; `kv-grid` → 2 kolom |
| ≤ 860px | Sidebar jadi drawer (fixed, 250px, geser dari kiri); panel utama tanpa margin/border/radius; breadcrumb dan teks search disembunyikan |
| ≤ 560px | `grid-4`, `form-grid`, `choice` → 1 kolom; OTP box mengecil; blok QR vertikal |

Animasi dimatikan saat `prefers-reduced-motion: reduce`.

---

## 7. Layout Sidebar

### 7.1 Anatomi

```text
 --bg ──────────────────────────────────────────────────────────────────────────────
│ SIDEBAR 236px            │ ┌ MAIN (panel, margin 8px, radius 12, border, shadow) ────┐
│ ┌──┐ App                 │ │ TOPBAR 54px                                             │
│ └──┘ v1.2.0 · abc123     │ │ [▯] │ Dashboard   (spacer)  [🔍 Search menu… Ctrl K] [↻] [☾] (A) │
│                          │ ├─────────────────────────────────────────────────────────┤
│ ▣ Dashboard   ← active   │ │ VIEW (scroll)                                           │
│                          │ │  .page                                                  │
│ PINNED              ˅    │ │   .page-head  h1 + p                 [page-actions]     │
│ ◇ item            ×      │ │   .grid .grid-4   KPI cards                             │
│                          │ │   .grid .grid-2   card | card                           │
│ GROUP               ˅    │ │                                                         │
│ ◇ Link           12      │ │                                                         │
│ ◇ Link          3/3 ●    │ │                                                         │
│                          │ │                                                         │
│ ACCOUNT             ˅    │ │                                                         │
│ ◇ Profile                │ │                                                         │
│ ◇ Users  (admin only)    │ │                                                         │
│                          │ │                                                         │
│ ⓘ About                  │ └─────────────────────────────────────────────────────────┘
```

| Bagian | Spesifikasi |
|---|---|
| Grid | `.app { display: grid; grid-template-columns: 236px minmax(0, 1fr); height: 100dvh; }` |
| Sidebar | Tanpa latar sendiri (menyatu dengan `--bg`), padding `14px 10px 10px`, isi scroll di `.side-nav` |
| Brand | Ikon 30×30 radius 8, nama 15px/700 `--accent-text`, di bawahnya versi 10.5px `--muted` (versi · commit) |
| Main | Panel "melayang": `margin: 8px 8px 8px 0`, `--panel`, border, radius 12, `--shadow` |
| Topbar | 54px, border bawah, gap 8px |
| View | `flex: 1; overflow: auto` — hanya area ini yang scroll |

### 7.2 Perilaku

| Aksi | Hasil | Disimpan |
|---|---|---|
| Klik tombol panel (desktop) | `html.sidebar-collapsed`: kolom sidebar 0, main dapat margin kiri 8px | `localStorage.<app>_sidebar = "collapsed"` |
| Muat halaman | Script `<head>` memasang `sidebar-collapsed` sebelum render (tanpa kedip) | — |
| Klik tombol panel (≤860px) | `html.sidebar-open`: drawer 250px meluncur dari kiri dengan `--shadow-pop` | Tidak disimpan |
| Klik di area main / pindah halaman (≤860px) | Drawer tertutup | — |
| Klik judul grup | Grup dilipat (chevron berputar -90°) | `localStorage.<app>_groups` (array id grup) |

CSS lengkap: [Lampiran A](#lampiran-a-maincss) bagian *Shell*, *Sidebar*, *Narrow screens*. Markup: [Lampiran B](#lampiran-b-indexhtml--authhtml).

---

## 8. Layout Menu

### 8.1 Struktur sidebar (urutan tetap)

| Urutan | Grup | Judul grup | Isi |
|---|---|---|---|
| 1 | `main` | tanpa judul | **Dashboard** (ikon `overview`) |
| 2 | `pinned` | `Pinned` | Item yang di-pin user (opsional); hanya muncul jika ada. Tombol `×` (unpin) muncul saat hover |
| 3… | grup domain | Nama area aplikasi (mis. `Data`, `Reports`, `Settings`) | Halaman inti aplikasi |
| n-1 | `account` | `Account` | **Profile** (`user`), **Users** (`users`, hanya admin). Grup hilang jika auth dimatikan |
| n | `about` | tanpa judul | **About** (`info`) |

Anatomi `.side-link` (tinggi 32px, radius 8px):

```text
[icon 16px, warna --accent]  [name, ellipsis]  [count 11px --muted]  [state dot 7px ok/danger]  [unpin × saat hover]
```

- Link aktif: latar `--accent-soft`, teks `--accent-text`, 600, `--shadow`.
- Hover: latar `--hover`, teks `--text`.
- Judul grup: 11px/600, `--muted`, `letter-spacing: .04em`, chevron di kanan, bisa diklik untuk melipat.
- Count/badge di link hanya untuk angka yang berguna (jumlah item, `ok/total` + dot kesehatan).
- Menu ditulis **deklaratif** di array `NAV` ([Lampiran C](#lampiran-c-javascript-inti)): `{ id, title?, auth?, links: [{ view, label, icon, keywords?, admin? }] }`. Command palette otomatis mengambil halaman dari `NAV`.

### 8.2 Topbar (urutan kiri → kanan)

| # | Elemen | Kelas | Keterangan |
|---|---|---|---|
| 1 | Toggle sidebar | `.icon-btn` + ikon `panel` | Collapse (desktop) / drawer (mobile) |
| 2 | Breadcrumb | `.crumbs` | Garis kiri, `Group › Sub › **Current**`; link di tengah warna `--muted` |
| 3 | Scope chip (opsional) | `.scope-chip` | Filter global aktif, pill aksen, klik untuk menghapus |
| 4 | Spacer | `.spacer` | |
| 5 | Tombol search | `.search` | Lebar `min(280px, 30vw)`, ikon `search`, teks "Search menu...", `kbd` "Ctrl K" / "⌘K" (Mac). Membuka command palette |
| 6 | Refresh | `.icon-btn` + `refresh` | Muat ulang data view aktif |
| 7 | Tema | `.icon-btn` + `moon`/`sun` | Toggle light/dark |
| 8 | Avatar | `.avatar` | Lingkaran 32px, huruf pertama display name, `--accent-soft`/`--accent-text` |

### 8.3 Menu avatar (dropdown `.menu`)

Auth aktif:

```text
Ari · Administrator        ← .menu-label (muted, tidak bisa diklik)
👤 Profile
👥 Users                   ← hanya admin
ⓘ About
⎋ Sign out
```

Auth nonaktif: label `Authentication is disabled` + `About`.

Aturan `.menu`: min-width 170px, padding 4px, item 30px radius 6px, `--shadow-pop`, max-height 60vh (scroll), menutup saat klik di luar, otomatis membuka ke atas bila tidak muat di bawah. Item terpilih: `.checked` (latar `--accent-soft`).

### 8.4 Command palette (Ctrl+K)

- Overlay di atas, `padding-top: 12vh`, kotak `min(600px, 100%)`, max-height 70vh.
- Input 46px (14px), daftar dikelompokkan: **Pages** → grup domain → **Actions** (`Toggle dark mode`, `Refresh data`, `Sign out`).
- Label grup: 10.5px uppercase, `letter-spacing: .06em`. Item 32px; terpilih `--accent-soft`.
- Footer: `↑↓ move` · `Enter open` · `Esc close`.
- Pencocokan: teks di label > kata di keywords/hint > huruf berurutan di label (fuzzy). Maksimal 60 hasil.
- Saat query kosong tampil per grup; saat mengetik diurutkan berdasarkan skor.

### 8.5 Shortcut keyboard

| Tombol | Aksi |
|---|---|
| `Ctrl+K` / `⌘K` | Buka command palette |
| `/` | Fokus kotak pencarian halaman (elemen dengan atribut `data-search`), atau buka palette bila tidak ada |
| `↑` `↓` `Enter` `Esc` | Navigasi palette |
| `Esc` | Tutup dialog |
| `Enter` | Submit form dialog; di dialog *type to confirm* hanya aktif setelah teks cocok |

---

## 9. Komponen UI

Semua CSS di [Lampiran A](#lampiran-a-maincss). Helper JS di `ui.js` ([Lampiran C](#lampiran-c-javascript-inti)).

### 9.1 Halaman

```text
.page
├── .page-head            h1 (22px) + p (--muted)          .page-actions (kanan: tombol, period)
├── .grid .grid-4         KPI / stat tile
├── .grid .grid-2 .mt     card lebar 1.6fr | card 1fr
├── .grid .grid-2e .mt    dua card sama lebar
└── p.page-foot           catatan bantuan dengan ikon help (opsional)
```

- Card: `.card` → `.card-head` (`h2` 14px + `p.card-sub` 11.5px muted, aksi di kanan) → isi.
- Loading pertama: skeleton (`.skel`) seukuran konten. Muat ulang: kelas `.refetching` pada `.page` (card jadi 55% opacity) sambil tetap menampilkan data lama.
- Gagal muat: `emptyState('alert', 'Could not load …', e.message)`.
- Kosong: `emptyState(icon, title, body, action?)` — ikon 30px, judul 14px, body maks 460px.

### 9.2 Tombol

| Kelas | Pakai untuk |
|---|---|
| `.btn` | Aksi sekunder |
| `.btn .btn-primary` | Satu aksi utama per area (latar `--accent`, teks putih) |
| `.btn .btn-danger` | Aksi destruktif, outline merah |
| `.btn .btn-danger .solid` | Konfirmasi destruktif di dialog |
| `.btn-sm` | Aksi di dalam card/tabel |
| `.btn-block` | Tombol penuh di form login |
| `.icon-btn` / `.icon-btn.sm` | Aksi ikon saja (wajib `title`) |
| `.link-btn` | Aksi teks (warna `--accent-text`, underline saat hover) |

Semua tombol: `:active` turun 1px; `:disabled` opacity .5. Saat proses: tombol di-disable dan teks diganti (`Checking...`, `Saving...`, `Verifying...`).

### 9.3 Form

- Label: `<label class="field"><span>Label</span><input class="input"></label>` (label 11.5px `--text-2`, gap 6px).
- Form dua kolom: `.form-grid` (`.full` untuk satu baris penuh), aksi di `.form-actions` (kanan).
- Error form: `p.form-error` (merah, 12px) di bawah tombol.
- Hint: `small.muted`.
- Nilai read-only: `.static`.
- Pilihan kartu: `.choice` > `label.choice-item` (radio, aktif `.on` dengan border aksen).
- Catatan: `p.note` (latar `--hover`) / `p.note.warn` (kuning), selalu dengan ikon 14px.

### 9.4 Data

| Komponen | Kelas | Aturan |
|---|---|---|
| Tabel | `table.table` dalam `.table-wrap` (atau `.card.table-card`) | Header 11px/600 muted; angka `.num` rata kanan tabular; baris bisa diklik `.table.clickable`; kolom sortable `th.sortable` |
| Badge | `.badge`, `.badge.ok`, `.badge.err`, `.badge.role-admin` | Tinggi 19px, radius 5px |
| Key–value | `dl.kv` (2 kolom), `.kv-grid` (4 kolom) | Label muted, nilai 600 |
| Meter | `.meter` > `div` (`.warn` >70%, `.bad` >90%) | Tinggi 8px |
| Stat tile | `.stat-tile` | Label + ikon aksen, nilai 24px, delta hijau/merah (`.delta.good` / `.delta.bad`) + sparkline opsional |
| Tabs | `.tabs` > `button.tab` (`.active`) | Segmented, latar `--hover`, tab aktif `--panel` + shadow |
| Periode | `.period` > `button` (`.active`) | Sama seperti tabs, lebih kecil (24px) |
| List baris | `.list-rows` > `.list-row` | Hover `--hover`, bisa diklik |
| Danger zone | `.card.danger-zone` > `.danger-row` | Border merah tipis; tiap aksi destruktif satu baris dengan penjelasan |

### 9.5 Feedback

| Komponen | Helper | Aturan |
|---|---|---|
| Toast | `toast(msg, 'ok' \| 'error')` | Kanan bawah; ok hilang 3.5 detik, error 6 detik; ikon `check`/`alert` |
| Konfirmasi | `confirmDialog({ title, body, confirmText, danger, typeToConfirm })` | Judul berupa pertanyaan ("Delete budi?"); body menjelaskan akibat; hapus permanen wajib `typeToConfirm` |
| Form dialog | `formDialog({ title, body, fields, submitText, danger, wide, cancel, onSubmit })` | Error dari `onSubmit` tampil di dialog tanpa menutup |
| Copy | `copy(text)` | Fallback `execCommand` untuk HTTP biasa; toast "Copied to clipboard" |
| Menu | `menu(anchor, items)` | Item `{ icon, text, onClick }` atau `{ label }` |

Gaya teks UI (Inggris): kalimat pendek, tanpa titik di judul/tombol, tombol berupa kata kerja spesifik (`Create user`, `Reset 2FA`, bukan `OK`/`Submit`), pesan error menyebut apa yang harus dilakukan.

---

## 10. Ikon

### 10.1 Ikon UI — Lucide

- Sumber: [Lucide](https://lucide.dev) (ISC). Di-inline sebagai sprite `<svg style="display:none">` berisi `<symbol id="i-<nama>" viewBox="0 0 24 24">` di **setiap** HTML yang memakainya.
- Pakai: `icon('name')` → `<svg class="icon"><use href="#i-name"/></svg>`. Ikon baru = tambah `<symbol id="i-name">` di HTML.
- Gaya: `.icon { width: 16px; height: 16px; fill: none; stroke: currentColor; stroke-width: 1.75; stroke-linecap: round; stroke-linejoin: round; }`.
- Ukuran konteks: tombol 14px, chip/tab 13px, stat tile 14–20px, empty state 30px (stroke 1.4), breadcrumb 12px.
- Ikon brand pihak ketiga (GitHub) dari Simple Icons (CC0) dengan kelas `.filled`.
- Kredit wajib di halaman About dan README: *JetBrains Mono (SIL OFL) dan ikon Lucide (ISC)*.
- Warna ikon mengikuti `currentColor`, sehingga otomatis benar di light dan dark.

Pemetaan ikon baku:

| Makna | Ikon |
|---|---|
| Dashboard | `overview` |
| Profile / user | `user` |
| Users | `users` |
| About / info | `info` |
| Sign out | `logout` |
| Search | `search` |
| Refresh | `refresh` |
| Tema | `moon` / `sun` |
| Toggle sidebar | `panel` |
| Tambah / edit / hapus | `plus` / `pencil` / `trash` |
| Aksi lain | `more` |
| Password / 2FA / keamanan | `key` / `shield` / `lock` |
| Bantuan / recovery | `help` |
| Sukses / error | `check` / `alert` |
| Salin | `copy` |
| Unduh / unggah | `download` / `upload` |
| Kosong | `inbox` |
| Tren naik / turun | `trend-up` / `trend-down` |

### 10.2 Ikon aplikasi — gaya macOS

Sumber vektor di `build/appicon.svg`, diekspor ke `web/static/appicon.png` 512px (sidebar, login, About, `apple-touch-icon`) dan `web/static/favicon.png` 64px.

| Unsur | Spesifikasi |
|---|---|
| Canvas | `viewBox="0 0 512 512"` |
| Bentuk | Squircle (rounded square) |
| Latar | Gradient diagonal cerah, 3–4 stop (contoh: `#4facfe` → `#3b82f6` → `#4f46e5` → `#4338ca`) |
| Kilap | Gradient putih dari atas: opacity .28 → .06 → 0 |
| Rim | Gradient bevel diagonal: putih .45 → .1 → hitam .2 |
| Bayangan | Dua `feDropShadow`: `dy 24, blur 28, opacity .32` + `dy 6, blur 10, opacity .18` |
| Objek | Simbol 3D sederhana yang menggambarkan fungsi aplikasi, dengan shading gradient |
| Tema | Ikon punya latar sendiri, jadi tampil sama di light dan dark |

Ekspor PNG:

```bash
rsvg-convert -w 512 -h 512 build/appicon.svg -o web/static/appicon.png
rsvg-convert -w 64  -h 64  build/appicon.svg -o web/static/favicon.png
```

---

## 11. Fitur Dasar

Setiap proyek yang punya dashboard web wajib memiliki fitur di bawah.

### 11.1 Wajib

| Fitur | Perilaku | Kode |
|---|---|---|
| Setup awal | Run pertama diarahkan ke `/setup`: akun admin → pertanyaan recovery → 2FA | `auth-page.js` (Lampiran D) |
| Login bertahap + 2FA | [§12](#12-login--autentikasi) | `auth-page.js`, `internal/auth` |
| Peran admin / operator | Operator bisa semua kecuali kelola user | middleware auth |
| Halaman **Users** (admin) | Tambah (password sementara acak), edit, ubah peran, reset password, reset 2FA, atur recovery admin lain, hapus (*type to confirm*), bagikan detail login | `views/users.js` |
| Halaman **Profile** | Ubah nama tampilan & username, ganti password, tampilkan ulang QR 2FA (butuh password, sembunyi otomatis 2 menit), atur pertanyaan recovery (admin) | `views/profile.js` |
| Halaman **Dashboard** | Ringkasan: baris KPI (`grid-4`) lalu card grafik/tabel; pilihan periode di `page-actions` | `views/dashboard.js` |
| Halaman **About** | Hero (ikon 64px, deskripsi, badge versi & lisensi), *Why it exists*, *What it does*, *Technology*, *Author* (GitHub, email), *Built with AI assistance*, *Credits* (font & ikon) | `views/about.js` |
| Sidebar + menu | [§7](#7-layout-sidebar), [§8](#8-layout-menu) | `app.js` |
| Command palette | Ctrl+K, halaman + aksi | `palette.js` |
| Light / dark mode | [§5.6](#56-mekanisme-light--dark-mode) | `index.html`, `auth.html`, `app.js` |
| Responsif | Drawer ≤860px | `main.css` |
| Status muat/kosong/error | Skeleton, `emptyState`, toast | `ui.js` |
| Refresh | Tombol topbar + `refresh()` per view | `app.js` |
| Versi di UI | `version · commit` di bawah nama brand, dari `GET /api/v1/config` | `app.js` |
| Health | `GET /healthz` publik `{"status":"ok"}` | handler |
| Auth bisa dimatikan | `APP_AUTH_ENABLED=false` → semua request dianggap admin anonim | `internal/auth` |

### 11.2 Opsional (pakai bila relevan)

| Fitur | Perilaku |
|---|---|
| Pin item ke sidebar | Grup *Pinned*, disimpan di `localStorage.<app>_pins` |
| Scope global | Chip di topbar untuk membatasi semua halaman ke satu cakupan; klik untuk menghapus |
| Ekspor | `GET …/export?format=csv\|json`, dibatasi jumlah maksimum; unduh lewat `api.download()` |
| Backup & restore | Unduh satu arsip `.tar.gz`, unggah untuk restore (aman diulang, tanpa duplikasi, tidak menghapus data) |
| Danger zone | Hapus sebagian data dan reset total, dengan *type to confirm* |
| Live update | WebSocket; indikator titik hijau (tersambung) / kuning (menunggu) |
| Zona waktu server | Semua tanggal dan jam di UI mengikuti zona waktu server (env), bukan browser |
| Grafik | SVG inline: batang bertumpuk, garis, sparkline, share bar; tooltip bisa dengan keyboard; tombol *Chart / Table* |

Aturan grafik: teks selalu warna teks (`--text`, `--muted`), hanya mark yang berwarna; gridline `--grid`; batang maksimal 24px; nilai di tooltip didahulukan; setiap grafik punya tampilan tabel.

### 11.3 Pola kode halaman (view)

- Satu file per halaman di `js/views/<nama>.js`, didaftarkan di `VIEWS` dan `NAV` (`app.js`).
- Kontrak modul:

```js
export function mount(root) {
  // build DOM, start loading
  return {
    update(params) {}, // dipanggil setiap URL params berubah
    refresh() {},      // tombol Refresh / palette
    destroy() {},      // hentikan timer, listener, request
  };
}
```

- Flag `alive` dicek setelah setiap `await` agar halaman yang sudah ditinggal tidak menulis DOM.
- Parameter filter halaman ada di URL (`patchRoute({ days: '7' })`), bukan di variabel global.
- Pitfall: `el.replaceChildren(list)` dengan array menghasilkan teks `[object HTMLDivElement]`. Pakai `...list` atau bungkus dengan `h()` (yang meratakan array).

### 11.4 Konvensi API

| Aturan | Nilai |
|---|---|
| Prefix | `/api/v1` |
| Router | `mux.HandleFunc("GET /api/v1/items", h.listItems)` |
| Error | `{"error": "pesan untuk manusia"}` dengan status HTTP yang tepat |
| 401 | `{"error": "sign in required", "auth_required": true}`; frontend redirect ke `/login?next=<path>` dan kembali ke `#view` yang sama |
| 403 | Non-admin ke `/api/v1/users*`: `only administrators can manage users` |
| Halaman | `GET /{$}` → `index.html`, `GET /login` dan `GET /setup` → `auth.html`, semua `no-cache` |
| Publik tanpa sesi | `/healthz`, `/api/v1/auth/*`, aset di `isAsset()` |
| Konfigurasi UI | `GET /api/v1/config` → `{version, commit, ...}` |

---

## 12. Login & Autentikasi

Spesifikasi ini berlaku untuk semua proyek. Nilai keamanan **tidak boleh** dilonggarkan.

### 12.1 Komponen backend

| Komponen | Tanggung jawab |
|---|---|
| `internal/auth` Manager | Simpan user (file JSON), hash bcrypt, TOTP, langkah login bertahap dengan temp token, recovery, token sesi HMAC, lockout |
| Middleware `Wrap` | Lewatkan `/healthz`, `/api/v1/auth/*`, aset publik; redirect halaman tanpa sesi ke `/setup` atau `/login?next=`; balas 401 JSON untuk API; tolak `/api/v1/users*` untuk non-admin (403); perpanjang sesi; taruh identitas user di context (`IdentityOf(r)`) |
| Handler auth | Endpoint [§12.7](#127-endpoint) |
| Handler users & profile | Kelola user (admin) dan akun sendiri |

### 12.2 Alur

**Setup awal** (`/setup`, satu kartu lebar dengan 3 langkah bertanda ikon; langkah selesai jadi hijau, yang belum 45% opacity):

1. **Administrator account** — username (default `admin`), password + checklist aturan, konfirmasi.
2. **Recovery question** — pilih dari daftar tetap, jawaban ≥ 3 karakter.
3. **Two-factor sign-in** — QR (+ kunci teks dan tombol *Copy key*), masukkan 6 digit → `Enable 2FA and sign in`.

**Login** (`/login`):

```text
Sign in (username + password)
   │ POST /auth/login/credentials
   ▼
server menentukan langkah berikut (temp_token berlaku 5 menit):
   ├─ totp     → "Two-factor authentication": 6 kotak OTP
   ├─ password → "Choose a new password" (akun baru / direset admin)
   ├─ enroll   → "Set up two-factor sign-in": QR baru (2FA direset)
   └─ done     → cookie sesi diset, redirect ke ?next (hanya path lokal) + #fragment
```

- Langkah bisa berantai (mis. `password` lalu `enroll` lalu `done`).
- Chip akun (`username` + tombol *Change*) tampil di setiap langkah lanjutan.
- Temp token kedaluwarsa → kembali ke form password dengan catatan kuning *"Your sign-in took too long. Enter your password again."* (server membalas `401 {"expired": true}`; frontend juga memasang timer sesuai `expires_in`).
- Di bawah form login: *"You stay signed in on this browser for 30 days."* dengan ikon `lock`.

**Recovery** (link *Forgot your password or lost your phone?*):

1. Masukkan username. Catatan: *Administrators answer their recovery question. Operators: ask an administrator to reset your account.*
2. Jawab pertanyaan **plus** satu faktor lain (pilihan kartu):
   - *I forgot my password* → konfirmasi dengan kode 2FA → pilih password baru.
   - *I lost my phone* → konfirmasi dengan password → scan QR baru.
3. Jawaban saja **tidak pernah** cukup untuk mereset apa pun.

### 12.3 Parameter keamanan

| Parameter | Nilai |
|---|---|
| Hash password & jawaban recovery | bcrypt |
| TOTP | RFC 6238, SHA1, 6 digit, periode 30 detik, toleransi ±1 langkah (±30 detik) |
| Secret TOTP | 20 byte acak, base32 tanpa padding (32 karakter) |
| URI | `otpauth://totp/<Issuer>:<username>?secret=…&issuer=<Issuer>&algorithm=SHA1&digits=6&period=30` |
| QR | PNG 256px (error correction Medium), dikirim sebagai data URL |
| Replay TOTP | Kode yang sudah dipakai ditolak (counter terakhir disimpan per user) |
| Temp token langkah login/recovery | 5 menit, maks 5 percobaan per langkah |
| Lockout | 5 gagal dalam 10 menit → dikunci 5 menit, **per klien** (hop pertama `X-Forwarded-For`) **dan per username** (`user:<nama>`) |
| Pesan lockout | `too many failed attempts, try again in <durasi>` |
| Sesi | Token `base64url(json).base64url(HMAC-SHA256)`, kunci HMAC disimpan di file auth (sesi tetap valid setelah restart) |
| Umur sesi | 30 hari, diterbitkan ulang otomatis bila sudah lebih dari 24 jam (sliding) |
| Cookie | `<app>_session`, `Path=/`, `HttpOnly`, `SameSite=Lax`, `Secure` bila HTTPS atau `X-Forwarded-Proto: https` |
| Pencabutan sesi | Ganti password atau reset oleh admin menaikkan `SessionGen` → semua sesi user itu berakhir |
| Script / API | `POST /api/v1/auth/login` `{username, password, code}` → token untuk `Authorization: Bearer <token>` |
| Redirect aman | `next` hanya path lokal (bukan `//…`, bukan `/\…`) |
| Admin terakhir | Tidak bisa dihapus atau diturunkan perannya |
| Admin tanpa recovery | Toast pengingat setelah login; badge merah *Set recovery question* di halaman Users |

### 12.4 Kebijakan password

Satu aturan, sama di server (`checkPassword`) dan browser (`passwordProblem` / `passwordRules`):

- Minimal **8 karakter**, maksimal **72 byte** (batas bcrypt).
- Wajib ada **huruf kecil**, **huruf besar**, **angka**, dan **simbol**.
- Contoh di placeholder dan pesan: `Qawsed#1477`.
- Checklist di bawah input password mencentang hijau saat tiap aturan terpenuhi: `8+ characters`, `lowercase`, `uppercase`, `number`, `symbol`.
- Berlaku di setup, user baru, reset, dan ganti password. Password lama tidak dicek ulang.
- Password sementara dibuat `randomPassword()` (14 karakter, tanpa karakter mirip, selalu memenuhi aturan).

### 12.5 Peran

| Peran | Label UI | Hak |
|---|---|---|
| `admin` | Administrator / badge *Admin* (warna aksen) | Semua fitur + menu Users |
| `operator` | Operator / badge *Operator* | Semua fitur kecuali Users |

- Server: middleware menolak `/api/v1/users*` untuk non-admin (403).
- Frontend: link Users dan item palette Users hanya dirender untuk admin; route `#/users` untuk non-admin diarahkan ke dashboard.
- User baru / direset admin login dengan password sementara, lalu wajib ganti password dan memasang 2FA.

### 12.6 Pertanyaan recovery (id tetap)

| id | Teks |
|---|---|
| `first_pet` | What was the name of your first pet? |
| `birth_city` | In which city were you born? |
| `first_school` | What was the name of your first school? |
| `childhood_friend` | What is the first name of your childhood best friend? |
| `mother_maiden` | What is your mother's maiden name? |
| `first_car` | What was the make and model of your first car? |
| `favorite_teacher` | What was the name of your favorite teacher? |
| `first_job` | Where did you work for your first job? |

Jawaban dinormalisasi (huruf kecil, spasi dirapatkan) sebelum di-bcrypt. Id tidak pernah diubah agar jawaban tetap cocok antar rilis. Recovery mandiri hanya untuk admin; operator direset admin.

### 12.7 Endpoint

| Metode | Path | Fungsi |
|---|---|---|
| `GET` | `/api/v1/auth/status` | `{auth_enabled, authenticated, setup_needed, user}` |
| `GET` | `/api/v1/auth/questions` | Daftar pertanyaan recovery |
| `POST` | `/api/v1/auth/setup/init` | Buat secret + QR untuk setup |
| `POST` | `/api/v1/auth/setup/complete` | `{username, password, secret, code, question, answer}` |
| `POST` | `/api/v1/auth/login/credentials` | `{username, password}` → langkah berikut |
| `POST` | `/api/v1/auth/login/2fa` | `{temp_token, code}` |
| `POST` | `/api/v1/auth/login/password` | `{temp_token, password}` |
| `POST` | `/api/v1/auth/login/enroll` | `{temp_token, code}` |
| `POST` | `/api/v1/auth/login` | Login satu langkah untuk skrip → token |
| `POST` | `/api/v1/auth/recover/start` | `{username}` → pertanyaan + temp token |
| `POST` | `/api/v1/auth/recover/verify` | `{temp_token, answer, mode: password\|2fa, code, password}` |
| `POST` | `/api/v1/auth/logout` | Hapus cookie |
| `GET` `POST` | `/api/v1/users` | *(admin)* daftar / tambah `{username, display_name, role, password}` |
| `PATCH` `DELETE` | `/api/v1/users/{id}` | *(admin)* ubah / hapus |
| `POST` | `/api/v1/users/{id}/password` | *(admin)* reset password sementara |
| `POST` | `/api/v1/users/{id}/2fa/reset` | *(admin)* reset 2FA |
| `PUT` | `/api/v1/users/{id}/recovery` | *(admin)* recovery admin lain `{question, answer}` |
| `GET` `PATCH` | `/api/v1/profile` | Profil sendiri |
| `POST` | `/api/v1/profile/password` | `{current, password}` |
| `POST` | `/api/v1/profile/2fa` | `{password}` → QR + secret |
| `PUT` | `/api/v1/profile/recovery` | *(admin)* `{question, answer, password}` |

Balasan langkah login: `{step, temp_token, username, expires_in, qr_data_url?, secret?}`.

Objek user: `{id, username, display_name, role, has_2fa, has_recovery, recovery_question, must_change_password, last_login_at, created_at}`.

### 12.8 Tampilan halaman login

```text
                                                          [☾]   ← .auth-theme (fixed kanan atas)

                    ┌────────────────────────────────────┐
                    │        [icon 34] App               │  ← .auth-brand, garis bawah
                    │                                    │
                    │              Sign in               │  ← .auth-title h1 20px
                    │         Welcome back to App        │     p --text-2
                    │                                    │
                    │  Username                          │
                    │  [______________________________]  │  ← input 36px
                    │  Password                          │
                    │  [______________________________]  │
                    │  [           Continue           ]  │  ← .btn-primary .btn-block 38px
                    │                                    │
                    │ Forgot your password or lost your phone? │  ← .link-btn
                    │ 🔒 You stay signed in on this browser for 30 days. │
                    └────────────────────────────────────┘  ← .auth-card 440px (wide 560px), radius 14

 © 2026 App                                            <tagline singkat>   ← .auth-footer
```

- Tema light/dark sama dengan dashboard (script `<head>` dan tombol tema sendiri).
- Kotak OTP: 6 input `inputmode="numeric"`, kotak pertama `autocomplete="one-time-code"`; mengetik pindah otomatis, Backspace/←/→ berpindah, paste 6 digit mengisi semua, lengkap 6 digit langsung submit.
- Blok QR: gambar 140px berlatar putih + *Cannot scan? Enter this key:* + kode + *Copy key*.
- Aplikasi authenticator yang disebut: Google Authenticator, Authy, 1Password.

### 12.9 Penyimpanan

| Item | Nilai |
|---|---|
| File | `auth.json` di volume persisten (`APP_AUTH_FILE`, default `/data/<app>/auth.json`, fallback `./data/auth.json`) |
| Isi | `version`, daftar user (hash password, secret TOTP, peran, hash jawaban recovery, `SessionGen`, waktu dibuat & login terakhir), kunci HMAC sesi |
| Migrasi | Naikkan `version` bila format berubah; format lama dimigrasikan saat dimuat |
| Semua admin terkunci | Hapus `auth.json` lalu setup ulang (data aplikasi tidak terpengaruh) |

---

## 13. Penulisan README.md

Bahasa **Indonesia** (sapaan formal "Anda"); istilah teknis tetap Inggris dan boleh dimiringkan (*port-forwarding*, *merge-sort*). README menjelaskan masalah dulu, baru solusi, lalu cara memakai.

### 13.1 Gaya

| Unsur | Standar |
|---|---|
| Header | `<div align="center">` berisi ikon `build/appicon.png` lebar 128, `# Nama`, kepanjangan/tagline **tebal**, satu paragraf deskripsi, lalu badge |
| Badge | shields.io `style=flat`; pakai `logo=` + `logoColor=white` untuk teknologi. Urutan: bahasa → platform → engine/storage → ukuran → dependensi → lisensi |
| Pemisah | `---` di antara setiap section `##` |
| Heading | Tanpa emoji. `##` section, `### Langkah N: …` untuk tahapan, `#### N. …` untuk sub-tahap |
| Fitur | `- **Nama Fitur** — Penjelasan.` Sub-butir untuk rincian |
| Callout | Blockquote dengan satu emoji + label tebal: `> 📖 **Panduan Lengkap:**`, `> 🏢 **Panduan Internal:**`, `> 🔒 **Opsi …:**`, `> **Catatan:**` |
| Perintah | Blok ` ```bash ` dengan komentar `# 1. …` di atas tiap perintah |
| Output | Blok ` ```text ` terpisah, didahului kalimat "Hasilnya akan menampilkan …" |
| Diagram | ASCII box (`┌─┐│└┘▼`) di blok kode tanpa bahasa |
| Tabel | Untuk: peran komponen, berkas deployment, endpoint API, troubleshooting, env, tech stack |
| Tangkapan layar | Tampilkan light **dan** dark berdampingan, atau pakai `<picture>` dengan `prefers-color-scheme` |
| Struktur folder | Tree dengan komentar `#` rata kanan |
| Penutup | *Lisensi* lalu *Kredit* (penulis, motivasi, kredit AI, kredit font & ikon, satu kalimat penutup dengan ⭐) |

### 13.2 Urutan section

| # | Section | Isi |
|---|---|---|
| 1 | Header | Ikon, nama, tagline, deskripsi, badge |
| 2 | `## Kenapa <App>` | Skenario nyata + blok kode/contoh kegagalan + daftar masalah bernomor + "**<App>** dirancang untuk …" dengan butir solusi |
| 3 | `## Fitur Unggulan` | Daftar fitur |
| 4 | `## Tampilan` | Tangkapan layar light & dark (opsional) |
| 5 | `## Arsitektur: Bagaimana <App> Bekerja?` | Paragraf model + diagram ASCII + tabel peran komponen |
| 6 | `## Instalasi & Deploy` | `### Langkah 1..N`: image/binary, konfigurasi, jalankan, akses; verifikasi dengan contoh output |
| 7 | `## Setup Awal & Login` | `####` Setup Awal, Login Seterusnya, Pengguna & Peran, Lupa Password / HP Hilang, opsi mematikan auth |
| 8 | `## Menjalankan dari Sumber & Pengujian Lokal` | `### Prasyarat`, lalu `### 1..N` dengan perintah `make` |
| 9 | `## <Operasional domain>` | Mis. manajemen data, backup & restore (opsional) |
| 10 | `## Dokumentasi REST API` | Tabel `Metode \| Endpoint \| Deskripsi`, catatan pagination, `### Contoh Pemanggilan Curl:` |
| 11 | `## Troubleshooting` | Tabel `Gejala \| Penyebab & Solusi` |
| 12 | `## Konfigurasi Lingkungan (Environment Variables)` | Tabel `Variabel Lingkungan \| Nilai Bawaan \| Keterangan` |
| 13 | `## Tumpukan Teknologi` | Tabel `Komponen \| Pustaka / Versi \| Alasan Pemilihan` |
| 14 | `## Struktur Direktori` | Tree berkomentar |
| 15 | `## Lisensi` | Satu kalimat + link `LICENSE` |
| 16 | `## Kredit` | Penulis, motivasi, kredit AI, kredit font/ikon |

Panduan panjang (registry, operasional internal, integrasi) dipindah ke `docs/*.md` (Bahasa Indonesia, nama file `UPPER_SNAKE_CASE.md`) dan ditautkan lewat callout.

### 13.3 Template

````markdown
<div align="center">

<img src="build/appicon.png" width="128" alt="App">

# App

**Kepanjangan Nama atau Tagline Singkat**

Satu paragraf: apa aplikasi ini, masalah apa yang diselesaikan, dan apa yang membedakannya dari solusi umum.

[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker&logoColor=white)](https://www.docker.com)
[![Storage](https://img.shields.io/badge/Storage-Nama%20Engine-7952B3?style=flat)](https://link-engine)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

</div>

---

## Kenapa App

Saat <situasi nyata>, hal berikut hampir selalu terjadi:

```bash
$ perintah-yang-biasa-dipakai
# Hasil yang mengecewakan...
```

1. **Masalah pertama.**
2. **Masalah kedua** dengan penjelasan singkat.
3. **Solusi umum (X atau Y)** membutuhkan <biaya/kerumitan>.

**App** dirancang untuk menyelesaikan masalah ini:
- **Pendekatan 1** — penjelasan.
- **Pendekatan 2** — penjelasan.

---

## Fitur Unggulan

- **Nama Fitur** — Penjelasan apa yang dilakukan.
- **Dashboard Web Bawaan** — Mode terang & gelap, command palette (Ctrl+K), tanpa Node.js atau web server terpisah.
- **Autentikasi 2FA (Google Authenticator)** — Login bertahap dengan TOTP (RFC 6238), wizard setup awal dengan QR code.
- **Manajemen Pengguna (Admin & Operasional)** — Dua peran; admin bisa memulihkan akun sendiri lewat pertanyaan keamanan.

---

## Tampilan

| Mode Terang | Mode Gelap |
|---|---|
| ![Dashboard mode terang](docs/images/dashboard-light.png) | ![Dashboard mode gelap](docs/images/dashboard-dark.png) |

---

## Arsitektur: Bagaimana App Bekerja?

Paragraf yang menjelaskan model arsitektur.

```
┌──────────────────────────────┐
│  Komponen A                  │
│  • tugas 1                   │
└──────────────┬───────────────┘
               │ (protokol)
               ▼
┌──────────────────────────────┐
│  Komponen B                  │
└──────────────────────────────┘
```

| Peran | Pola Deployment | Lokasi Berjalan | Fungsi Utama |
|---|---|---|---|
| **`a`** | ... | ... | ... |

---

## Instalasi & Deploy

### Langkah 1: Siapkan Container Image

```bash
# 1. Build image
docker build -t your-registry/app:latest .

# 2. Push ke registry
docker push your-registry/app:latest
```

> 📖 **Panduan Lengkap Registry:** Lihat **[`docs/CONTAINER_REGISTRY_GUIDE.md`](docs/CONTAINER_REGISTRY_GUIDE.md)**.

### Langkah 2: Jalankan

```bash
docker run -d --name app -p 8080:8080 -v app-data:/data/app your-registry/app:latest
```

Hasilnya akan menampilkan container berjalan:
```text
CONTAINER ID   IMAGE                      STATUS         PORTS
3f2a1b0c9d8e   your-registry/app:latest   Up 5 seconds   0.0.0.0:8080->8080/tcp
```

### Langkah 3: Akses Dashboard

Buka peramban Anda di: **[`http://localhost:8080`](http://localhost:8080)**

---

## Setup Awal & Login

#### 1. Setup Awal (Onboarding Pertama Kali)
1. **Akun Administrator:** username dan password (minimal 8 karakter dengan huruf kecil, huruf besar, angka, dan simbol, contoh `Qawsed#1477`).
2. **Pertanyaan Pemulihan:** pilih pertanyaan dan isi jawaban (tidak peka huruf besar/kecil).
3. **Scan QR Code 2FA:** pindai dengan **Google Authenticator**, **Authy**, atau **1Password**, lalu masukkan 6 digit kode.

#### 2. Login Seterusnya (Alur Bertahap)
1. **Kredensial:** username dan password.
2. **Kode 2FA:** 6 digit dari aplikasi authenticator.
3. **Langkah tambahan bila perlu:** akun baru atau yang direset admin wajib mengganti password; akun yang 2FA-nya direset wajib scan QR baru.
4. **Sesi Aktif:** cookie `HttpOnly` bertanda tangan HMAC, berlaku **30 hari**, tetap valid walau aplikasi restart.

Setiap langkah login berlaku 5 menit. Setelah 5 kali gagal, klien dan akun dikunci 5 menit.

#### 3. Pengguna & Peran
| Peran | Hak akses |
|---|---|
| **Admin** | Semua fitur + menu **Users** |
| **Operasional** | Semua fitur kecuali menu Users |

#### 4. Lupa Password / HP Hilang
- **Admin:** klik *Forgot your password or lost your phone?*, jawab pertanyaan pemulihan, lalu konfirmasi dengan kode 2FA (lupa password) atau password (HP hilang).
- **Operasional:** minta admin melakukan reset lewat menu **Users**.

> 🔒 **Opsi Jaringan Tertutup:** set `APP_AUTH_ENABLED=false` untuk menonaktifkan login.

---

## Menjalankan dari Sumber & Pengujian Lokal

### Prasyarat
- **Go 1.26+** terpasang

### 1. Jalankan Aplikasi

```bash
make run
```
Aplikasi aktif di port `:8080` dan menyajikan dashboard web.

### 2. Menjalankan Unit Test

```bash
make test
```

---

## Dokumentasi REST API

| Metode | Endpoint | Deskripsi |
|---|---|---|
| `GET` | `/api/v1/items` | ... |
| `GET` | `/healthz` | Liveness probe publik |
| `GET` `POST` | `/api/v1/users` | *(Admin)* Daftar / tambah pengguna |

### Contoh Pemanggilan Curl:

```bash
# 1. Login sekali untuk mendapat token (kode = 6 digit dari authenticator)
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -d '{"username":"admin","password":"<password>","code":"123456"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')

# 2. Panggil API
curl -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/items"
```

Token berlaku 30 hari. Jika `APP_AUTH_ENABLED=false`, header `Authorization` tidak diperlukan.

---

## Troubleshooting

| Gejala | Penyebab & Solusi |
|---|---|
| ... | ... |

---

## Konfigurasi Lingkungan (Environment Variables)

| Variabel Lingkungan | Nilai Bawaan | Keterangan |
|---|---|---|
| `APP_PORT` | `8080` | Port dashboard web |
| `APP_AUTH_ENABLED` | `true` | Login + 2FA di dashboard |
| `APP_AUTH_FILE` | `/data/app/auth.json` | Lokasi data akun |

---

## Tumpukan Teknologi

| Komponen | Pustaka / Versi | Alasan Pemilihan |
|---|---|---|
| **Bahasa Utama** | Go **1.26+** | Kompilasi single binary |
| **Frontend UI** | HTML5, CSS3 kustom, Vanilla JS | Di-embed lewat `go:embed`, tanpa build-step Node |
| **Autentikasi** | bcrypt + TOTP (RFC 6238) | Password ter-hash dan login dua faktor |

---

## Struktur Direktori

```
app/
├── cmd/app/main.go                 # Entrypoint
├── internal/                       # Logika aplikasi
├── web/static/                     # Aset dashboard (HTML, CSS, JS, font, ikon)
├── build/appicon.svg               # Ikon vektor aplikasi bergaya macOS
├── Dockerfile                      # Multi-stage container build
├── Makefile                        # Otomasi build, test, dev runner
└── README.md
```

---

## Lisensi

Didistribusikan di bawah lisensi [MIT](LICENSE). Bebas digunakan, dimodifikasi, dan didistribusikan baik untuk keperluan pribadi maupun komersial.

---

## Kredit

**Penulis:** Nama Lengkap — [github.com/username](https://github.com/username) · [email@contoh.com](mailto:email@contoh.com)

Satu kalimat motivasi mengapa aplikasi ini dibuat.

Sebagian perancangan dan penulisan kode dibantu model AI; setiap usulan tetap ditinjau, diuji, dan disesuaikan secara manual.

Font JetBrains Mono Nerd Font (SIL OFL) dan ikon Lucide (ISC). Halaman **About** di dashboard memuat ringkasan yang sama.

Satu kalimat penutup. ⭐
````

---

## 14. CLAUDE.md

Setiap repo punya `CLAUDE.md` (Bahasa Inggris) dengan kerangka:

````markdown
# CLAUDE.md

Guidance for Claude Code when working in this repository.

## What this is
One paragraph: what the app does, its components, ports.
The README and `docs/` are written in Indonesian. Code, comments and commit messages are in English and use Conventional Commits.
UI, login and README follow MYSTANDARD.md.

## Commands
```bash
make build   # ...
make test    # ...
```

## Layout
```
cmd/<app>/main.go   entry
internal/...        one line per package
web/                go:embed of web/static; plain ES modules, NO build step
```

## Architecture notes and invariants
### <Topic>
- Rules that must stay true, and why.

### Auth
- Cookie name, auth file path, public paths in isAsset().

### Dashboard (web/static)
- Entry points, where icons live (a new icon('x') needs <symbol id="i-x"> in the HTML), pitfalls, rebuild after editing.

## Conventions
- slog only, net/http mux method patterns, minimal deps, short "why" comments.
````

---

## 15. Checklist Proyek Baru

### Tampilan

- [ ] Font `"<App> Mono"` di `web/static/fonts/`, preload Regular di `index.html` dan `auth.html`, `OFL.txt` ikut.
- [ ] Token warna **light dan dark** sama dengan [§5](#5-color-scheme-light--dark) (hanya aksen boleh diganti, kontras diperiksa).
- [ ] Script tema di `<head>` kedua HTML; default mengikuti sistem; pilihan tersimpan `<app>_theme`.
- [ ] Dashboard dan halaman login dicek di light **dan** dark.
- [ ] Shell: sidebar 236px, panel utama melayang, topbar 54px, drawer ≤860px.
- [ ] Menu disusun lewat `NAV` dengan urutan [§8.1](#81-struktur-sidebar-urutan-tetap).
- [ ] Topbar lengkap: toggle, breadcrumb, search Ctrl+K, refresh, tema, avatar.
- [ ] Semua ikon dari sprite Lucide; ikon aplikasi bergaya macOS di `build/appicon.svg` → `appicon.png` 512 & `favicon.png` 64.

### Fitur & login

- [ ] Halaman Dashboard, Profile, Users (admin), About.
- [ ] `/setup` 3 langkah, `/login` bertahap, recovery admin.
- [ ] Parameter keamanan [§12.3](#123-parameter-keamanan) tidak diubah; issuer TOTP dan nama cookie memakai nama proyek.
- [ ] Password policy identik di server dan `ui.js`.
- [ ] `isAsset()` memuat semua path statis publik.
- [ ] `GET /healthz` publik, `GET /api/v1/config` mengembalikan versi.
- [ ] `APP_AUTH_ENABLED=false` berfungsi.

### Dokumen

- [ ] README mengikuti [§13](#13-penulisan-readmemd), termasuk tangkapan layar light & dark.
- [ ] `CLAUDE.md` ada ([§14](#14-claudemd)).
- [ ] Kredit font & ikon di README dan About.
- [ ] Tidak ada nama starter `App` tersisa ([§1.2](#12-daftar-ganti-nama)).

---

# Bagian B — Lampiran Kode Starter

## Lampiran A: main.css

Stylesheet lengkap: font, token light & dark, shell, sidebar, komponen, halaman login, command palette, form, halaman About, dan breakpoint. Ganti `"App Mono"` sesuai nama aplikasi. Token ANSI dari [§5.2](#52-warna-severity-dan-grafik-opsional) ditambahkan hanya bila perlu.

### `web/static/styles/main.css`

```css
/* App dashboard. One accent (burnt orange), stone neutrals, 10px cards / 6px controls. */

/* JetBrains Mono Nerd Font (SIL OFL, see /fonts/OFL.txt): Nerd Font glyphs render too. */
@font-face {
  font-family: "App Mono";
  src: url("/fonts/JetBrainsMonoNerdFontMono-Regular.woff2") format("woff2");
  font-weight: 100 500; font-style: normal; font-display: swap;
}
@font-face {
  font-family: "App Mono";
  src: url("/fonts/JetBrainsMonoNerdFontMono-Bold.woff2") format("woff2");
  font-weight: 600 900; font-style: normal; font-display: swap;
}

:root {
  --bg: #f4f3f1;
  --panel: #ffffff;
  --panel-2: #fafaf9;
  --hover: #f1efec;
  --border: #e7e4e0;
  --border-strong: #d5d1cc;
  --text: #1c1917;
  --text-2: #4a4542;
  --muted: #7c756f;
  --accent: #c2410c;
  --accent-soft: #fcece2;
  --accent-text: #b13a0a;
  --danger: #c42b1c;
  --ok: #15803d;
  --shadow: 0 1px 2px rgb(41 37 36 / .06);
  --shadow-pop: 0 8px 28px rgb(41 37 36 / .14);

  /* severity levels: chart/mark colors (validated palette) and readable text variants */
  --lv-debug: #2f9e83; --lvt-debug: #1f7a64;
  --lv-info: #4f7fd9;  --lvt-info: #2f62c0;
  --lv-warn: #c98a0c;  --lvt-warn: #94620a;
  --lv-error: #d63d2b; --lvt-error: #b72f1f;
  --lv-fatal: #a3174f; --lvt-fatal: #a3174f;


  --radius: 10px;
  --radius-sm: 6px;
  --font: "App Mono", "JetBrainsMono Nerd Font Mono", "JetBrains Mono", "SF Mono", "Cascadia Code", ui-monospace, Menlo, Consolas, monospace;
  color-scheme: light;
}

[data-theme="dark"] {
  --bg: #121110;
  --panel: #1c1a19;
  --panel-2: #211f1d;
  --hover: #292624;
  --border: #302d2a;
  --border-strong: #423e3a;
  --text: #ece9e6;
  --text-2: #c4beb8;
  --muted: #948d86;
  --accent: #ea6a2a;
  --accent-soft: rgb(234 106 42 / .15);
  --accent-text: #f28b52;
  --danger: #e5584e;
  --ok: #4ade80;
  --shadow: 0 1px 2px rgb(0 0 0 / .3);
  --shadow-pop: 0 10px 30px rgb(0 0 0 / .45);

  --lv-debug: #319a80; --lvt-debug: #4fc0a2;
  --lv-info: #5282d8;  --lvt-info: #82a8ee;
  --lv-warn: #b88c14;  --lvt-warn: #e0b240;
  --lv-error: #d9475a; --lvt-error: #f07a86;
  --lv-fatal: #c0487e; --lvt-fatal: #e57fae;
  color-scheme: dark;
}

* { box-sizing: border-box; }
[hidden] { display: none !important; }
html, body { height: 100%; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font: 12.5px/1.5 var(--font);
  -webkit-font-smoothing: antialiased;
}
a { color: inherit; text-decoration: none; }
button, input, select { font: inherit; color: inherit; }
kbd {
  font: 10.5px var(--font);
  color: var(--muted);
  border: 1px solid var(--border);
  border-bottom-width: 2px;
  border-radius: 4px;
  padding: 0 5px;
  background: var(--panel);
}
mark { background: color-mix(in srgb, var(--accent) 28%, transparent); color: inherit; border-radius: 2px; }
.icon {
  width: 16px; height: 16px; flex: none;
  fill: none; stroke: currentColor; stroke-width: 1.75; stroke-linecap: round; stroke-linejoin: round;
}
.muted { color: var(--muted); }
.spacer { flex: 1; }
.only-dark { display: none; }
[data-theme="dark"] .only-dark { display: block; }
[data-theme="dark"] .only-light { display: none; }

/* ── Shell ── */
.app { display: grid; grid-template-columns: 236px minmax(0, 1fr); height: 100dvh; }
.sidebar-collapsed .app { grid-template-columns: 0 minmax(0, 1fr); }
.sidebar-collapsed .sidebar { visibility: hidden; }

.sidebar { display: flex; flex-direction: column; min-height: 0; padding: 14px 10px 10px; overflow: hidden; }
.brand { display: flex; align-items: center; gap: 10px; padding: 2px 8px 14px; }
.brand img { border-radius: 8px; }
.brand-text { display: flex; flex-direction: column; line-height: 1.2; min-width: 0; }
.brand-name { font-size: 15px; font-weight: 700; color: var(--accent-text); letter-spacing: -.01em; }
.brand-ver { font-size: 10.5px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }


.type-tag { font-size: 9.5px; color: var(--muted); border: 1px solid var(--border); border-radius: 4px; padding: 0 4px; line-height: 15px; }


.main {
  display: flex; flex-direction: column; min-width: 0; min-height: 0;
  margin: 8px 8px 8px 0; background: var(--panel);
  border: 1px solid var(--border); border-radius: 12px; box-shadow: var(--shadow); overflow: hidden;
}
.sidebar-collapsed .main { margin-left: 8px; }
.topbar { display: flex; align-items: center; gap: 8px; height: 54px; padding: 0 12px; border-bottom: 1px solid var(--border); flex: none; }
.crumbs { display: flex; align-items: center; gap: 6px; min-width: 0; padding-left: 8px; margin-left: 4px; border-left: 1px solid var(--border); color: var(--muted); white-space: nowrap; overflow: hidden; }
.crumbs b { color: var(--text); font-weight: 600; }
.crumbs .icon { width: 12px; height: 12px; }
.crumbs span { overflow: hidden; text-overflow: ellipsis; }

.search {
  display: flex; align-items: center; gap: 8px; width: min(280px, 30vw); height: 32px; padding: 0 8px 0 10px; cursor: pointer;
  border: 1px solid var(--border); border-radius: 8px; background: var(--panel-2); color: var(--muted); text-align: left;
}
.search:hover { border-color: var(--border-strong); color: var(--text-2); }
.search:focus-visible { outline: 0; border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
.search-text { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.icon-btn {
  display: inline-grid; place-items: center; width: 32px; height: 32px; flex: none;
  border: 0; border-radius: 8px; background: none; color: var(--text-2); cursor: pointer;
}
.icon-btn:hover { background: var(--hover); color: var(--text); }
.icon-btn:active { transform: translateY(1px); }
.icon-btn:disabled { opacity: .4; cursor: default; background: none; }
.icon-btn.sm { width: 26px; height: 26px; }
.avatar {
  width: 32px; height: 32px; border-radius: 50%; border: 1px solid var(--border-strong); cursor: pointer;
  background: var(--accent-soft); color: var(--accent-text); font-weight: 700; margin-left: 4px;
}

.view { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; }

/* ── Controls ── */
.btn {
  display: inline-flex; align-items: center; justify-content: center; gap: 7px; height: 28px; padding: 0 11px;
  border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--panel);
  color: var(--text); cursor: pointer; white-space: nowrap; transition: background .15s, border-color .15s;
}
.btn:hover { background: var(--hover); border-color: var(--border-strong); }
.btn:active { transform: translateY(1px); }
.btn:disabled { opacity: .5; cursor: default; transform: none; }
.btn .icon { width: 14px; height: 14px; }
.btn-primary { background: var(--accent); border-color: var(--accent); color: #fff; }
.btn-primary:hover { background: color-mix(in srgb, var(--accent) 88%, #000); border-color: transparent; }
.btn-danger { color: var(--danger); border-color: color-mix(in srgb, var(--danger) 35%, var(--border)); }
.btn-danger:hover { background: color-mix(in srgb, var(--danger) 9%, transparent); border-color: var(--danger); }
.btn-danger.solid { background: var(--danger); color: #fff; border-color: var(--danger); }
.btn-sm { height: 26px; padding: 0 9px; font-size: 11.5px; }

.input, .select {
  height: 28px; padding: 0 9px; border: 1px solid var(--border); border-radius: var(--radius-sm);
  background: var(--panel); color: var(--text); outline: 0; min-width: 0;
}
.input:focus, .select:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
.select { padding-right: 24px; appearance: none; cursor: pointer; max-width: 170px; text-overflow: ellipsis;
  background-image: linear-gradient(45deg, transparent 50%, var(--muted) 50%), linear-gradient(135deg, var(--muted) 50%, transparent 50%);
  background-position: calc(100% - 13px) 50%, calc(100% - 9px) 50%; background-size: 4px 4px; background-repeat: no-repeat; }
/* A set filter stays white (the native option list takes the select's colors); only weight and border mark it. */
.select.has-value { border-color: var(--border-strong); color: var(--text); font-weight: 600; }
.select option { background: var(--panel); color: var(--text); font-weight: 400; }
.input-group { display: inline-flex; align-items: center; gap: 4px; }
.field { display: flex; flex-direction: column; gap: 6px; }
.field > span { font-size: 11.5px; color: var(--text-2); }
.field .input { height: 34px; }

.chip {
  display: inline-flex; align-items: center; gap: 6px; height: 28px; padding: 0 9px;
  border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--panel);
  color: var(--muted); font-size: 11px; cursor: pointer; white-space: nowrap;
}
.chip:hover { border-color: var(--border-strong); color: var(--text-2); }
.chip .dot { width: 7px; height: 7px; border-radius: 2px; background: var(--border-strong); }
.chip.active { color: var(--text); border-color: var(--border-strong); }
.chip.active .dot { background: var(--lvc); }
.chip .icon { width: 13px; height: 13px; }
.chip.live.active { color: var(--accent-text); border-color: color-mix(in srgb, var(--accent) 45%, var(--border)); background: var(--accent-soft); }
.chips { display: inline-flex; gap: 4px; }

.lv-DEBUG { --lvc: var(--lv-debug); --lvt: var(--lvt-debug); }
.lv-INFO  { --lvc: var(--lv-info);  --lvt: var(--lvt-info); }
.lv-WARN  { --lvc: var(--lv-warn);  --lvt: var(--lvt-warn); }
.lv-ERROR { --lvc: var(--lv-error); --lvt: var(--lvt-error); }
.lv-FATAL { --lvc: var(--lv-fatal); --lvt: var(--lvt-fatal); }
.lv {
  display: inline-block; min-width: 44px; text-align: center; font-size: 10px; font-weight: 700; letter-spacing: .03em;
  padding: 0 5px; border-radius: 4px; line-height: 17px; color: var(--lvt);
  background: color-mix(in srgb, var(--lvc) 14%, transparent);
}

.toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; padding: 8px 12px; border-bottom: 1px solid var(--border); flex: none; }
.toolbar .sep { width: 1px; height: 20px; background: var(--border); margin: 0 4px; }
.range { display: inline-flex; align-items: center; gap: 6px; color: var(--muted); }
.range .icon { width: 14px; height: 14px; }

.menu {
  position: fixed; z-index: 30; min-width: 170px; padding: 4px; background: var(--panel);
  border: 1px solid var(--border); border-radius: 8px; box-shadow: var(--shadow-pop);
}
.menu button, .menu .menu-label {
  display: flex; align-items: center; gap: 8px; width: 100%; height: 30px; padding: 0 10px;
  border: 0; background: none; border-radius: 6px; text-align: left; cursor: pointer;
}
.menu button:hover { background: var(--hover); }
.menu .menu-label { color: var(--muted); cursor: default; font-size: 11.5px; }

/* ── Pages ── */
.page { padding: 22px 26px 30px; }
.page-fill { flex: 1; min-height: 0; display: flex; flex-direction: column; }
.page-head { display: flex; align-items: flex-end; gap: 12px; margin-bottom: 18px; }
.page-head h1 { margin: 0; font-size: 22px; letter-spacing: -.02em; }
.page-head p { margin: 2px 0 0; color: var(--muted); }

.grid { display: grid; gap: 14px; }
.grid-4 { grid-template-columns: repeat(4, minmax(0, 1fr)); }
.grid-2 { grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr); }
.grid-2e { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.card { background: var(--panel); border: 1px solid var(--border); border-radius: var(--radius); padding: 16px 18px; box-shadow: var(--shadow); min-width: 0; }
.card h2 { margin: 0; font-size: 14px; }
.card .card-sub { color: var(--muted); margin: 2px 0 12px; font-size: 11.5px; }
.card-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; }

.stat { display: flex; gap: 14px; align-items: flex-start; }
.stat-icon { display: grid; place-items: center; width: 42px; height: 42px; border-radius: 10px; flex: none; background: var(--accent-soft); color: var(--accent-text); }
.stat-icon .icon { width: 20px; height: 20px; }
.stat-label { color: var(--muted); }
.stat-value { font-size: 26px; font-weight: 700; line-height: 1.15; letter-spacing: -.02em; }
.stat-sub { display: flex; align-items: center; gap: 5px; margin-top: 3px; font-size: 11.5px; color: var(--text-2); }
.stat-sub .icon { width: 13px; height: 13px; color: var(--ok); }
.stat-sub.bad .icon { color: var(--danger); }

.list-rows { display: flex; flex-direction: column; }
.list-row {
  display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 10px;
  padding: 7px 8px; margin: 0 -8px; border-radius: 6px; cursor: pointer;
}
.list-row:hover { background: var(--hover); }
.list-row .sub { color: var(--muted); font-size: 11px; }
.list-row .bar { height: 4px; border-radius: 2px; background: var(--lv-error); margin-top: 5px; }
.list-row.err-row { grid-template-columns: 76px 50px minmax(0, 1fr); font-size: 11.5px; }
.ellipsis { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }

.meter { height: 8px; border-radius: 4px; background: var(--hover); overflow: hidden; margin: 10px 0 6px; }
.meter > div { height: 100%; border-radius: 4px; background: var(--accent); }
.meter.warn > div { background: var(--lv-warn); }
.meter.bad > div { background: var(--danger); }
.kv { display: grid; grid-template-columns: auto 1fr; gap: 4px 14px; font-size: 12px; }
.kv dt { color: var(--muted); }
.kv dd { margin: 0; text-align: right; }

table.table { width: 100%; border-collapse: collapse; font-size: 12px; }
.table th { text-align: left; font-weight: 600; color: var(--muted); font-size: 11px; padding: 6px 8px; border-bottom: 1px solid var(--border); }
.table td { padding: 7px 8px; border-bottom: 1px solid var(--border); }
.table tr:last-child td { border-bottom: 0; }
.table .num { text-align: right; font-variant-numeric: tabular-nums; }
.table .inline-bar { display: block; height: 4px; border-radius: 2px; background: var(--accent); opacity: .7; min-width: 2px; }
.table-wrap { overflow-x: auto; }

.danger-zone { border-color: color-mix(in srgb, var(--danger) 30%, var(--border)); }
.danger-row { display: flex; align-items: center; justify-content: space-between; gap: 14px; flex-wrap: wrap; padding: 12px 0; }
.danger-row + .danger-row { border-top: 1px solid var(--border); }
.danger-row p { margin: 2px 0 0; color: var(--muted); font-size: 11.5px; }

.empty { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 6px; padding: 60px 20px; text-align: center; color: var(--muted); }
.empty .icon { width: 30px; height: 30px; stroke-width: 1.4; margin-bottom: 6px; }
.empty h3 { margin: 0; color: var(--text); font-size: 14px; }
.empty p { margin: 0 0 8px; max-width: 460px; }
.error-text { color: var(--danger); }

.skel { border-radius: 4px; background: linear-gradient(90deg, var(--hover) 25%, var(--panel-2) 50%, var(--hover) 75%); background-size: 200% 100%; animation: shimmer 1.3s infinite linear; }
.skel-row { height: 12px; margin: 6px 14px; }
@keyframes shimmer { from { background-position: 200% 0; } to { background-position: -200% 0; } }

/* ── Overlays ── */
.overlay { position: fixed; inset: 0; z-index: 40; display: grid; place-items: center; padding: 16px; background: rgb(12 10 9 / .45); }
.dialog { width: min(440px, 100%); padding: 20px; background: var(--panel); border: 1px solid var(--border); border-radius: 12px; box-shadow: var(--shadow-pop); }
.dialog h3 { margin: 0 0 6px; font-size: 15px; }
.dialog-body { margin: 0 0 16px; color: var(--text-2); }
.dialog .field { margin-bottom: 16px; }
.dialog-actions { display: flex; justify-content: flex-end; gap: 8px; }


.toasts { position: fixed; right: 16px; bottom: 16px; z-index: 60; display: flex; flex-direction: column; gap: 8px; }
.toast {
  display: flex; align-items: center; gap: 8px; max-width: 380px; padding: 10px 12px;
  background: var(--panel); border: 1px solid var(--border); border-radius: 8px; box-shadow: var(--shadow-pop);
  animation: toast-in .2s ease-out; transition: opacity .2s, transform .2s;
}
.toast .icon { color: var(--ok); }
.toast.error .icon { color: var(--danger); }
.toast.out { opacity: 0; transform: translateY(6px); }
@keyframes toast-in { from { opacity: 0; transform: translateY(6px); } }

@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after { animation: none !important; transition: none !important; }
}

/* ── Narrow screens ── */
@media (max-width: 1100px) {
  .grid-4 { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .grid-2, .grid-2e { grid-template-columns: minmax(0, 1fr); }
}
@media (max-width: 860px) {
  .app, .sidebar-collapsed .app { grid-template-columns: minmax(0, 1fr); }
  .sidebar {
    position: fixed; inset: 0 auto 0 0; z-index: 35; width: 250px; background: var(--bg);
    border-right: 1px solid var(--border); transform: translateX(-100%); transition: transform .2s; visibility: visible !important;
  }
  .sidebar-open .sidebar { transform: none; box-shadow: var(--shadow-pop); }
  .main, .sidebar-collapsed .main { margin: 0; border-radius: 0; border: 0; }
  .search kbd, .crumbs { display: none; }
  .search { width: auto; flex: 1; }
  .page { padding: 16px; }
}
@media (max-width: 560px) {
  .grid-4 { grid-template-columns: minmax(0, 1fr); }
}

/* ── Sidebar: grouped links (Kite-style) ── */
.side-nav { flex: 1; min-height: 0; overflow-y: auto; padding: 2px 0 8px; scrollbar-width: thin; }
.side-group + .side-group { margin-top: 12px; }
.side-group-title {
  display: flex; align-items: center; justify-content: space-between; width: 100%; padding: 4px 10px 5px;
  border: 0; background: none; cursor: pointer; color: var(--muted); font-size: 11px; font-weight: 600; letter-spacing: .04em;
}
.side-group-title:hover { color: var(--text-2); }
.side-group-title .icon { width: 13px; height: 13px; transition: transform .15s; }
.side-group.collapsed .side-group-title .icon { transform: rotate(-90deg); }
.side-group.collapsed .side-links { display: none; }
.side-links { display: flex; flex-direction: column; gap: 1px; }
.side-link {
  display: flex; align-items: center; gap: 10px; height: 32px; padding: 0 10px;
  border-radius: 8px; color: var(--text-2); transition: background .15s, color .15s;
}
.side-link:hover { background: var(--hover); color: var(--text); }
.side-link .icon { color: var(--accent); }
.side-link.active { background: var(--accent-soft); color: var(--accent-text); font-weight: 600; box-shadow: var(--shadow); }
.side-link .name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.side-link .count { font-size: 11px; color: var(--muted); font-weight: 400; }
.side-link .state { width: 7px; height: 7px; border-radius: 50%; background: var(--ok); flex: none; }
.side-link .state.bad { background: var(--danger); }
.side-link .unpin { display: none; place-items: center; width: 20px; height: 20px; border: 0; border-radius: 5px; background: none; color: var(--muted); cursor: pointer; }
.side-link .unpin .icon { color: inherit; width: 12px; height: 12px; }
.side-link:hover .unpin { display: inline-grid; }
.side-link .unpin:hover { background: var(--border); color: var(--text); }
.scope-chip {
  display: inline-flex; align-items: center; gap: 6px; height: 24px; padding: 0 6px 0 8px; margin-left: 4px; flex: none;
  border: 1px solid color-mix(in srgb, var(--accent) 35%, var(--border)); border-radius: 12px;
  background: var(--accent-soft); color: var(--accent-text); font-size: 11px; cursor: pointer; white-space: nowrap;
}
.scope-chip .icon { width: 12px; height: 12px; }
.scope-chip:hover { border-color: var(--accent); }
.menu { max-height: 60vh; overflow-y: auto; }
.menu button.checked { color: var(--accent-text); font-weight: 600; background: var(--accent-soft); }
.crumbs a { color: var(--muted); }
.crumbs a:hover { color: var(--text); }

/* ── Pages, detail, tables ── */
.mt { margin-top: 14px; }
.grid.stack { grid-template-columns: minmax(0, 1fr); align-content: start; }
.page-actions { display: flex; align-items: center; gap: 8px; margin-left: auto; flex-wrap: wrap; }
.page-sub { display: flex; align-items: center; gap: 8px; color: var(--muted); }
.page-detail .page-head { align-items: flex-start; margin-bottom: 14px; }
.page-detail h1 { font-size: 24px; }
.page.fill { flex: 1; min-height: 0; display: flex; flex-direction: column; padding-bottom: 14px; }
.page.fill .tab-body { flex: 1; min-height: 0; display: flex; flex-direction: column; }
.tabs { display: inline-flex; align-self: flex-start; gap: 2px; padding: 3px; margin-bottom: 14px; background: var(--hover); border-radius: 9px; }
.tab { display: inline-flex; align-items: center; gap: 6px; height: 28px; padding: 0 12px; border: 0; border-radius: 7px; background: none; color: var(--text-2); cursor: pointer; }
.tab:hover { color: var(--text); }
.tab.active { background: var(--panel); color: var(--text); box-shadow: var(--shadow); font-weight: 600; }
.badge { display: inline-flex; align-items: center; gap: 5px; height: 19px; padding: 0 7px; border-radius: 5px; font-size: 11px; background: var(--hover); color: var(--text-2); white-space: nowrap; }
.tab .badge { height: 17px; padding: 0 5px; background: var(--border); }
.badge.ok { background: color-mix(in srgb, var(--ok) 15%, transparent); color: color-mix(in srgb, var(--ok) 80%, var(--text)); }
.badge.err { background: color-mix(in srgb, var(--danger) 13%, transparent); color: var(--danger); }
.err-text, .num.err { color: var(--lvt-error); font-weight: 600; }
.kv-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px 22px; }
.kv-grid .k { color: var(--muted); font-size: 11.5px; }
.kv-grid .v { margin-top: 3px; font-weight: 600; overflow-wrap: anywhere; }
.table-card { padding: 4px 8px 6px; }
.table-foot { padding: 8px; color: var(--muted); font-size: 11.5px; }
.table.clickable tbody tr { cursor: pointer; }
.table.clickable tbody tr:hover td { background: var(--hover); }
.table th.sortable { cursor: pointer; user-select: none; }
.table th.sortable:hover, .table th.sorted { color: var(--text); }
.table .strong { font-weight: 600; color: var(--text); }
.table .sub { color: var(--muted); font-size: 11px; }
.table td { vertical-align: middle; }
.pin .icon { width: 14px; height: 14px; color: var(--muted); }
.pin.on .icon { color: var(--accent); }

/* ── Sign-in and setup pages ── */
.auth-body { background: var(--bg); }
.auth-page { min-height: 100dvh; display: grid; grid-template-rows: 1fr auto; }
.auth-theme { position: fixed; top: 16px; right: 16px; }
.auth-main { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 22px; padding: 56px 16px; }
.auth-brand {
  display: flex; align-items: center; justify-content: center; gap: 10px; margin: -4px 0 20px; padding-bottom: 18px;
  border-bottom: 1px solid var(--border); font-size: 18px; font-weight: 700; letter-spacing: -.02em; color: var(--accent-text);
}
.auth-brand img { border-radius: 9px; }
.auth-card { width: min(440px, 100%); padding: 28px 30px 30px; background: var(--panel); border: 1px solid var(--border); border-radius: 14px; box-shadow: var(--shadow-pop); }
.auth-card.wide { width: min(560px, 100%); }
.auth-title { text-align: center; margin-bottom: 22px; }
.auth-title h1 { margin: 0; font-size: 20px; }
.auth-title p { margin: 6px 0 0; color: var(--text-2); }
.auth-form { display: flex; flex-direction: column; gap: 14px; }
.auth-form .field .input { height: 36px; }
.btn-block { width: 100%; height: 38px; }
.steps { display: flex; flex-direction: column; gap: 20px; }
.step { display: grid; grid-template-columns: 36px minmax(0, 1fr); column-gap: 12px; }
.step-icon { display: grid; place-items: center; width: 36px; height: 36px; border-radius: 50%; border: 1.5px solid var(--accent); color: var(--accent); }
.step-icon .icon { width: 18px; height: 18px; }
.step.done .step-icon { background: var(--ok); border-color: var(--ok); color: #fff; }
.step.done .step-head h2 { color: color-mix(in srgb, var(--ok) 80%, var(--text)); }
.step.pending { opacity: .45; }
.step-head h2 { margin: 0; font-size: 14px; }
.step-head p { margin: 2px 0 0; color: var(--muted); font-size: 11.5px; }
.step-body { grid-column: 2; margin-top: 14px; }
.qr { display: flex; gap: 14px; align-items: center; padding: 12px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--panel-2); }
.qr img { width: 140px; height: 140px; flex: none; padding: 6px; border-radius: 6px; background: #fff; }
.qr code { display: block; margin: 4px 0 8px; font-size: 11px; word-break: break-all; color: var(--text); }
.otp-row { display: flex; justify-content: center; gap: 8px; }
.otp-box {
  width: 44px; height: 50px; text-align: center; font-size: 20px; font-weight: 600; outline: 0;
  border: 1px solid var(--border-strong); border-radius: 8px; background: var(--panel); color: var(--text);
}
.otp-box:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
.form-error { min-height: 16px; margin: 0; color: var(--danger); font-size: 12px; text-align: center; }
.account-chip { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 9px 12px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--panel-2); }
.account-chip .icon { width: 14px; height: 14px; vertical-align: -2px; color: var(--muted); }
.link-btn { padding: 0; border: 0; background: none; color: var(--accent-text); cursor: pointer; }
.link-btn:hover { text-decoration: underline; }
.auth-hint { margin: 18px 0 0; text-align: center; color: var(--muted); font-size: 11.5px; }
.auth-hint .icon { width: 13px; height: 13px; vertical-align: -2px; }
.auth-footer { display: flex; justify-content: space-between; gap: 12px; padding: 16px max(16px, 8vw); border-top: 1px solid var(--border); background: var(--panel); color: var(--muted); font-size: 12px; }

@media (max-width: 1100px) {
  .kv-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 560px) {
  .otp-box { width: 38px; height: 46px; }
  .qr { flex-direction: column; }
}

/* ── Chart tokens and stat tiles ── */
:root { --spark: #b5aea7; --grid: #eeebe8; }
[data-theme="dark"] { --spark: #5e5853; --grid: #2a2826; }
.stat-tile { display: flex; flex-direction: column; gap: 4px; }
.stat-tile .stat-label { display: flex; align-items: center; gap: 6px; min-width: 0; font-size: 11.5px; }
.stat-tile .stat-label span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.stat-tile .stat-label .icon { width: 14px; height: 14px; color: var(--accent); }
.stat-main { display: flex; align-items: flex-end; justify-content: space-between; gap: 10px; min-width: 0; }
.stat-tile .stat-value { font-size: 24px; }
.stat-tile .stat-sub { margin-top: 0; }
.spark { display: inline-block; line-height: 0; }
.delta { display: inline-flex; align-items: center; gap: 3px; font-weight: 600; color: var(--text-2); }
.delta .icon { width: 13px; height: 13px; }
.delta.good { color: var(--ok); }
.delta.bad { color: var(--danger); }
.stat-sub .delta .icon { color: inherit; }
.share-bar { display: flex; gap: 2px; height: 12px; margin: 6px 0 12px; border-radius: 4px; overflow: hidden; }
.share-bar span { min-width: 3px; }
.share-table td, .share-table th { padding: 5px 6px; }
.share-table .swatch { vertical-align: -1px; }
.period { display: inline-flex; gap: 2px; padding: 3px; background: var(--hover); border-radius: 8px; }
.period button { height: 24px; padding: 0 10px; border: 0; border-radius: 6px; background: none; color: var(--text-2); cursor: pointer; font-size: 11.5px; }
.period button.active { background: var(--panel); color: var(--text); font-weight: 600; box-shadow: var(--shadow); }
.trend-cell { display: flex; align-items: center; justify-content: flex-end; gap: 8px; }
.refetching .card { opacity: .55; transition: opacity .15s; }
.list-row.top-err { grid-template-columns: minmax(0, 1fr) auto auto; }
.num-pair { display: flex; flex-direction: column; align-items: flex-end; line-height: 1.25; }
.num-pair small { color: var(--muted); font-size: 10.5px; font-weight: 400; }

/* ── Search box in a page toolbar ── */
.tool-search {
  display: inline-flex; align-items: center; gap: 6px; height: 28px; width: min(300px, 100%); padding: 0 8px; cursor: text;
  border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--panel); color: var(--muted);
}
.tool-search:focus-within { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
.tool-search .icon { width: 14px; height: 14px; }
.tool-search input { flex: 1; min-width: 0; border: 0; outline: 0; background: none; color: var(--text); }

/* ── Command palette (Ctrl+K) ── */
.palette-overlay { place-items: start center; padding-top: 12vh; }
.palette {
  display: flex; flex-direction: column; width: min(600px, 100%); max-height: 70vh; overflow: hidden;
  background: var(--panel); border: 1px solid var(--border); border-radius: 12px; box-shadow: var(--shadow-pop);
}
.palette-search { display: flex; align-items: center; gap: 10px; height: 46px; padding: 0 14px; border-bottom: 1px solid var(--border); color: var(--muted); flex: none; }
.palette-input { flex: 1; min-width: 0; border: 0; outline: 0; background: none; color: var(--text); font-size: 14px; }
.palette-list { flex: 1; min-height: 0; overflow-y: auto; padding: 6px; }
.palette-group { padding: 8px 10px 4px; font-size: 10.5px; font-weight: 600; letter-spacing: .06em; text-transform: uppercase; color: var(--muted); }
.palette-item {
  display: flex; align-items: center; gap: 10px; width: 100%; height: 32px; padding: 0 10px;
  border: 0; border-radius: 6px; background: none; color: var(--text-2); text-align: left; cursor: pointer;
}
.palette-item .icon { color: var(--muted); }
.palette-item.sel { background: var(--accent-soft); color: var(--accent-text); }
.palette-item.sel .icon { color: var(--accent); }
.palette-item .label { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.palette-item .hint { color: var(--muted); font-size: 11px; white-space: nowrap; }
.palette-empty { padding: 26px; text-align: center; color: var(--muted); }
.palette-foot { display: flex; gap: 14px; padding: 7px 14px; border-top: 1px solid var(--border); background: var(--panel-2); color: var(--muted); font-size: 11px; flex: none; }

/* ── Forms: profile, users, dialogs ── */
.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 14px; }
.form-grid .full { grid-column: 1 / -1; }
.form-grid .field .input, .form-grid .field .select { height: 32px; }
.form-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 14px; }
.static { display: flex; align-items: center; height: 32px; color: var(--text-2); }
.select.wide { max-width: none; width: 100%; }
.dialog.wide { width: min(520px, 100%); }
.dialog .form-error { text-align: left; margin: -6px 0 10px; }
.dialog .field small { font-size: 11px; }
.note, .auth-note {
  display: flex; gap: 8px; align-items: flex-start; margin: 0; padding: 9px 12px;
  border-radius: var(--radius-sm); background: var(--hover); color: var(--text-2); font-size: 12px;
}
.note .icon, .auth-note .icon { width: 14px; height: 14px; margin-top: 2px; flex: none; }
.note.warn, .auth-note.warn { background: color-mix(in srgb, var(--lv-warn) 14%, transparent); color: var(--lvt-warn); }
.mt-sm { margin-top: 12px; }
.tf-row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.cred { display: flex; flex-direction: column; gap: 8px; padding: 12px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--panel-2); }
.cred code { font-size: 13px; font-weight: 700; color: var(--text); user-select: all; }
.badge.role-admin { background: var(--accent-soft); color: var(--accent-text); }
.page-foot { display: flex; gap: 8px; align-items: flex-start; margin: 12px 2px 0; color: var(--muted); font-size: 11.5px; }
.page-foot .icon { width: 14px; height: 14px; flex: none; margin-top: 1px; }

/* Password rule checklist (ticks turn green while typing) */
.pw-rules { display: flex; flex-wrap: wrap; gap: 4px 12px; margin: -6px 0 0; padding: 0; list-style: none; font-size: 11px; color: var(--muted); }
.pw-rules li { display: inline-flex; align-items: center; gap: 4px; }
.pw-rules .icon { width: 12px; height: 12px; opacity: .3; }
.pw-rules li.ok { color: var(--ok); }
.pw-rules li.ok .icon { opacity: 1; }
.form-grid .pw-rules { margin: -4px 0 0; }
.status-cell { display: flex; flex-wrap: wrap; gap: 4px; align-items: center; }
.badge-btn { border: 0; cursor: pointer; font: inherit; font-size: 11px; }
.badge-btn .icon { width: 11px; height: 11px; }
.badge-btn:hover { filter: brightness(.95); text-decoration: underline; }

/* ── About page ── */
.icon .filled { fill: currentColor; stroke: none; }
.about-hero { display: flex; align-items: flex-start; gap: 18px; }
.about-hero img { border-radius: 14px; flex: none; }
.about-hero h2 { margin: 0 0 6px; font-size: 18px; }
.about-hero p { margin: 0; color: var(--text-2); max-width: 70ch; }
.about-badges { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 12px; }
.about-badges .badge { gap: 5px; }
.about-badges .icon { width: 12px; height: 12px; }
.about-list { display: grid; grid-template-columns: minmax(0, 1fr); gap: 12px; margin: 0; }
.about-list dt { font-weight: 600; color: var(--text); }
.about-list dd { margin: 2px 0 0; color: var(--text-2); }
.about-text { margin: 0 0 12px; color: var(--text-2); }
.about-author { display: flex; align-items: center; gap: 12px; }
.about-avatar {
  display: grid; place-items: center; width: 46px; height: 46px; flex: none; border-radius: 50%;
  background: var(--accent-soft); color: var(--accent-text); font-weight: 700; font-size: 15px;
}
.about-links { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 14px; }

/* ── Sign-in extras ── */
.field.center > span { text-align: center; }
.auth-links { text-align: center; font-size: 12px; }
.auth-card .select.wide { height: 36px; }
.choice { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.choice-item { display: flex; gap: 8px; align-items: flex-start; padding: 9px 10px; border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; font-size: 12px; }
.choice-item input { margin: 2px 0 0; accent-color: var(--accent); }
.choice-item small { display: block; color: var(--muted); font-size: 11px; }
.choice-item.on { border-color: var(--accent); background: var(--accent-soft); }
@media (max-width: 860px) {
  .scope-chip span { max-width: 90px; overflow: hidden; text-overflow: ellipsis; }
  .search-text { display: none; }
  .search { width: auto; flex: none; }
}
@media (max-width: 560px) {
  .form-grid, .choice { grid-template-columns: minmax(0, 1fr); }
}
```

## Lampiran B: index.html & auth.html

Sprite ikon Lucide: hapus `<symbol>` yang tidak dipakai atau tambah yang baru dengan pola yang sama.

### `web/static/index.html`

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>App</title>
  <link rel="icon" type="image/png" href="/favicon.png">
  <link rel="apple-touch-icon" href="/appicon.png">
  <link rel="preload" href="/fonts/JetBrainsMonoNerdFontMono-Regular.woff2" as="font" type="font/woff2" crossorigin>
  <link rel="stylesheet" href="/styles/main.css">
  <script>
    try {
      document.documentElement.dataset.theme = localStorage.getItem('app_theme') ||
        (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
      if (localStorage.getItem('app_sidebar') === '"collapsed"') document.documentElement.classList.add('sidebar-collapsed');
    } catch (e) {}
  </script>
</head>
<body>
<!-- Icon glyphs from the Lucide icon set (ISC license), inlined because the dashboard ships without a build step. -->
<svg xmlns="http://www.w3.org/2000/svg" style="display:none">
  <symbol id="i-overview" viewBox="0 0 24 24"><rect width="7" height="9" x="3" y="3" rx="1"/><rect width="7" height="5" x="14" y="3" rx="1"/><rect width="7" height="9" x="14" y="12" rx="1"/><rect width="7" height="5" x="3" y="16" rx="1"/></symbol>
  <symbol id="i-file-text" viewBox="0 0 24 24"><path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M10 9H8"/><path d="M16 13H8"/><path d="M16 17H8"/></symbol>
  <symbol id="i-compare" viewBox="0 0 24 24"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M12 3v18"/></symbol>
  <symbol id="i-storage" viewBox="0 0 24 24"><line x1="22" x2="2" y1="12" y2="12"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/><line x1="6" x2="6.01" y1="16" y2="16"/><line x1="10" x2="10.01" y1="16" y2="16"/></symbol>
  <symbol id="i-folder" viewBox="0 0 24 24"><path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/></symbol>
  <symbol id="i-layers" viewBox="0 0 24 24"><path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/><path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.65"/><path d="m22 12.65-9.17 4.16a2 2 0 0 1-1.66 0L2 12.65"/></symbol>
  <symbol id="i-box" viewBox="0 0 24 24"><path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/><path d="m3.3 7 8.7 5 8.7-5"/><path d="M12 22V12"/></symbol>
  <symbol id="i-rocket" viewBox="0 0 24 24"><path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z"/><path d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z"/><path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0"/><path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5"/></symbol>
  <symbol id="i-grid" viewBox="0 0 24 24"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M3 12h18"/><path d="M12 3v18"/></symbol>
  <symbol id="i-server" viewBox="0 0 24 24"><rect width="20" height="8" x="2" y="2" rx="2"/><rect width="20" height="8" x="2" y="14" rx="2"/><line x1="6" x2="6.01" y1="6" y2="6"/><line x1="6" x2="6.01" y1="18" y2="18"/></symbol>
  <symbol id="i-search" viewBox="0 0 24 24"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></symbol>
  <symbol id="i-refresh" viewBox="0 0 24 24"><path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/></symbol>
  <symbol id="i-sun" viewBox="0 0 24 24"><circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/></symbol>
  <symbol id="i-moon" viewBox="0 0 24 24"><path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/></symbol>
  <symbol id="i-panel" viewBox="0 0 24 24"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M9 3v18"/></symbol>
  <symbol id="i-chevron-right" viewBox="0 0 24 24"><path d="m9 18 6-6-6-6"/></symbol>
  <symbol id="i-chevron-left" viewBox="0 0 24 24"><path d="m15 18-6-6 6-6"/></symbol>
  <symbol id="i-chevron-down" viewBox="0 0 24 24"><path d="m6 9 6 6 6-6"/></symbol>
  <symbol id="i-updown" viewBox="0 0 24 24"><path d="m7 15 5 5 5-5"/><path d="m7 9 5-5 5 5"/></symbol>
  <symbol id="i-x" viewBox="0 0 24 24"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></symbol>
  <symbol id="i-download" viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></symbol>
  <symbol id="i-copy" viewBox="0 0 24 24"><rect width="14" height="14" x="8" y="8" rx="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></symbol>
  <symbol id="i-trash" viewBox="0 0 24 24"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></symbol>
  <symbol id="i-logout" viewBox="0 0 24 24"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" x2="9" y1="12" y2="12"/></symbol>
  <symbol id="i-play" viewBox="0 0 24 24"><polygon points="6 3 20 12 6 21 6 3"/></symbol>
  <symbol id="i-pause" viewBox="0 0 24 24"><rect x="14" y="4" width="4" height="16" rx="1"/><rect x="6" y="4" width="4" height="16" rx="1"/></symbol>
  <symbol id="i-check" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/></symbol>
  <symbol id="i-alert" viewBox="0 0 24 24"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4"/><path d="M12 17h.01"/></symbol>
  <symbol id="i-crosshair" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><line x1="22" x2="18" y1="12" y2="12"/><line x1="6" x2="2" y1="12" y2="12"/><line x1="12" x2="12" y1="6" y2="2"/><line x1="12" x2="12" y1="22" y2="18"/></symbol>
  <symbol id="i-clock" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></symbol>
  <symbol id="i-inbox" viewBox="0 0 24 24"><polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/></symbol>
  <symbol id="i-link" viewBox="0 0 24 24"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></symbol>
  <symbol id="i-activity" viewBox="0 0 24 24"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></symbol>
  <symbol id="i-pin" viewBox="0 0 24 24"><path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/></symbol>
  <symbol id="i-history" viewBox="0 0 24 24"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/><path d="M12 7v5l4 2"/></symbol>
  <symbol id="i-arrow-right" viewBox="0 0 24 24"><path d="M5 12h14"/><path d="m12 5 7 7-7 7"/></symbol>
  <symbol id="i-user" viewBox="0 0 24 24"><circle cx="12" cy="8" r="5"/><path d="M20 21a8 8 0 0 0-16 0"/></symbol>
  <symbol id="i-users" viewBox="0 0 24 24"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></symbol>
  <symbol id="i-key" viewBox="0 0 24 24"><path d="m15.5 7.5 2.3 2.3a1 1 0 0 0 1.4 0l2.1-2.1a1 1 0 0 0 0-1.4L19 4"/><path d="m21 2-9.6 9.6"/><circle cx="7.5" cy="15.5" r="5.5"/></symbol>
  <symbol id="i-shield" viewBox="0 0 24 24"><path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/></symbol>
  <symbol id="i-lock" viewBox="0 0 24 24"><rect width="18" height="11" x="3" y="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></symbol>
  <symbol id="i-help" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/></symbol>
  <symbol id="i-upload" viewBox="0 0 24 24"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" x2="12" y1="3" y2="15"/></symbol>
  <symbol id="i-archive" viewBox="0 0 24 24"><rect width="20" height="5" x="2" y="3" rx="1"/><path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8"/><path d="M10 12h4"/></symbol>
  <symbol id="i-pencil" viewBox="0 0 24 24"><path d="M21.17 6.81a1 1 0 0 0-3.99-3.99L3.84 16.17a2 2 0 0 0-.5.83l-1.32 4.35a.5.5 0 0 0 .62.62l4.35-1.32a2 2 0 0 0 .83-.5z"/><path d="m15 5 4 4"/></symbol>
  <symbol id="i-plus" viewBox="0 0 24 24"><path d="M5 12h14"/><path d="M12 5v14"/></symbol>
  <symbol id="i-more" viewBox="0 0 24 24"><circle cx="12" cy="12" r="1"/><circle cx="12" cy="5" r="1"/><circle cx="12" cy="19" r="1"/></symbol>
  <symbol id="i-checkmark" viewBox="0 0 24 24"><path d="M20 6 9 17l-5-5"/></symbol>
  <symbol id="i-trend-up" viewBox="0 0 24 24"><polyline points="22 7 13.5 15.5 8.5 10.5 2 17"/><polyline points="16 7 22 7 22 13"/></symbol>
  <symbol id="i-trend-down" viewBox="0 0 24 24"><polyline points="22 17 13.5 8.5 8.5 13.5 2 7"/><polyline points="16 17 22 17 22 11"/></symbol>
  <symbol id="i-minus" viewBox="0 0 24 24"><path d="M5 12h14"/></symbol>
  <symbol id="i-table" viewBox="0 0 24 24"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M3 9h18"/><path d="M3 15h18"/><path d="M9 3v18"/></symbol>
  <symbol id="i-chart" viewBox="0 0 24 24"><path d="M3 3v16a2 2 0 0 0 2 2h16"/><path d="M7 16h.01"/><path d="M11 16V11"/><path d="M15 16V7"/><path d="M19 16v-4"/></symbol>
  <symbol id="i-percent" viewBox="0 0 24 24"><line x1="19" x2="5" y1="5" y2="19"/><circle cx="6.5" cy="6.5" r="2.5"/><circle cx="17.5" cy="17.5" r="2.5"/></symbol>
  <symbol id="i-info" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></symbol>
  <symbol id="i-mail" viewBox="0 0 24 24"><rect width="20" height="16" x="2" y="4" rx="2"/><path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"/></symbol>
  <symbol id="i-sparkles" viewBox="0 0 24 24"><path d="M9.937 15.5A2 2 0 0 0 8.5 14.063l-6.135-1.582a.5.5 0 0 1 0-.962L8.5 9.936A2 2 0 0 0 9.937 8.5l1.582-6.135a.5.5 0 0 1 .963 0L14.063 8.5A2 2 0 0 0 15.5 9.937l6.135 1.581a.5.5 0 0 1 0 .964L15.5 14.063a2 2 0 0 0-1.437 1.437l-1.582 6.135a.5.5 0 0 1-.963 0z"/><path d="M20 3v4"/><path d="M22 5h-4"/></symbol>
  <!-- GitHub mark from Simple Icons (CC0), filled instead of stroked. -->
  <symbol id="i-github" viewBox="0 0 24 24"><path class="filled" d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/></symbol>
</svg>

<div class="app">
  <aside class="sidebar" id="sidebar">
    <a class="brand" href="#/dashboard">
      <img src="/appicon.png" alt="" width="30" height="30">
      <span class="brand-text">
        <span class="brand-name">App</span>
        <span class="brand-ver" id="brand-ver">&nbsp;</span>
      </span>
    </a>
    <nav class="side-nav" id="side-nav" aria-label="Main"></nav>
  </aside>

  <main class="main">
    <header class="topbar">
      <button class="icon-btn" id="toggle-sidebar" type="button" title="Toggle sidebar"><svg class="icon"><use href="#i-panel"/></svg></button>
      <div class="crumbs" id="crumbs"></div>
      <button class="scope-chip" id="scope-chip" type="button" hidden></button>
      <div class="spacer"></div>
      <button class="search" id="palette" type="button" title="Go to a page or run an action">
        <svg class="icon"><use href="#i-search"/></svg>
        <span class="search-text">Search menu...</span>
        <kbd id="search-kbd">Ctrl K</kbd>
      </button>
      <button class="icon-btn" id="refresh" type="button" title="Refresh"><svg class="icon"><use href="#i-refresh"/></svg></button>
      <button class="icon-btn" id="theme" type="button" title="Toggle theme">
        <svg class="icon only-light"><use href="#i-moon"/></svg>
        <svg class="icon only-dark"><use href="#i-sun"/></svg>
      </button>
      <button class="avatar" id="avatar" type="button" title="Account"></button>
    </header>
    <div class="view" id="view"></div>
  </main>
</div>

<div class="toasts" id="toasts" aria-live="polite"></div>
<script type="module" src="/js/app.js"></script>
</body>
</html>
```

### `web/static/auth.html`

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Sign in - App</title>
  <link rel="icon" type="image/png" href="/favicon.png">
  <link rel="preload" href="/fonts/JetBrainsMonoNerdFontMono-Regular.woff2" as="font" type="font/woff2" crossorigin>
  <link rel="stylesheet" href="/styles/main.css">
  <script>
    try {
      document.documentElement.dataset.theme = localStorage.getItem('app_theme') ||
        (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    } catch (e) {}
  </script>
</head>
<body class="auth-body">
<!-- Icon glyphs from the Lucide icon set (ISC license). -->
<svg xmlns="http://www.w3.org/2000/svg" style="display:none">
  <symbol id="i-sun" viewBox="0 0 24 24"><circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/></symbol>
  <symbol id="i-moon" viewBox="0 0 24 24"><path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/></symbol>
  <symbol id="i-user" viewBox="0 0 24 24"><circle cx="12" cy="8" r="5"/><path d="M20 21a8 8 0 0 0-16 0"/></symbol>
  <symbol id="i-shield" viewBox="0 0 24 24"><path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/></symbol>
  <symbol id="i-lock" viewBox="0 0 24 24"><rect width="18" height="11" x="3" y="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></symbol>
  <symbol id="i-key" viewBox="0 0 24 24"><path d="m15.5 7.5 2.3 2.3a1 1 0 0 0 1.4 0l2.1-2.1a1 1 0 0 0 0-1.4L19 4"/><path d="m21 2-9.6 9.6"/><circle cx="7.5" cy="15.5" r="5.5"/></symbol>
  <symbol id="i-help" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/></symbol>
  <symbol id="i-checkmark" viewBox="0 0 24 24"><path d="M20 6 9 17l-5-5"/></symbol>
  <symbol id="i-check" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/></symbol>
  <symbol id="i-alert" viewBox="0 0 24 24"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4"/><path d="M12 17h.01"/></symbol>
  <symbol id="i-copy" viewBox="0 0 24 24"><rect width="14" height="14" x="8" y="8" rx="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></symbol>
</svg>

<div class="auth-page">
  <button class="icon-btn auth-theme" id="theme" type="button" title="Toggle theme">
    <svg class="icon only-light"><use href="#i-moon"/></svg>
    <svg class="icon only-dark"><use href="#i-sun"/></svg>
  </button>
  <main class="auth-main">
    <div class="auth-card" id="card-wrap">
      <div class="auth-brand"><img src="/appicon.png" width="34" height="34" alt=""><span>App</span></div>
      <div id="card"><div class="skel" style="height:240px"></div></div>
    </div>
  </main>
  <footer class="auth-footer">
    <span id="copyright">App</span>
    <span>Short tagline</span>
  </footer>
</div>
<div class="toasts" id="toasts" aria-live="polite"></div>
<script type="module" src="/js/auth-page.js"></script>
</body>
</html>
```

## Lampiran C: JavaScript inti

Halaman Dashboard dan halaman baru lainnya dimulai dari template `views/items.js`. Profile, Users, dan About sudah siap pakai; di `about.js` cukup isi konstanta di bagian atas.

### `web/embed.go`

```go
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

func sub() fs.FS {
	s, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("failed to get static sub-filesystem: " + err.Error())
	}
	return s
}

// StaticHandler serves the embedded dashboard assets.
func StaticHandler() http.Handler {
	return http.FileServer(http.FS(sub()))
}

// Page serves one embedded HTML file (used for / , /login and /setup).
// Pages are never cached so a new release is picked up immediately.
func Page(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := fs.ReadFile(sub(), name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(b)
	})
}
```

### `web/static/js/api.js`

```js
// REST client for the server API. Authentication is the HttpOnly session
// cookie set by /login, so requests carry no token of their own.

export class ApiError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

export function qs(params = {}) {
  const u = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '') u.set(k, v);
  }
  const s = u.toString();
  return s ? '?' + s : '';
}

// Session gone: go to the sign-in page and come back to the same view after.
export function redirectToLogin() {
  location.href = '/login?next=' + encodeURIComponent(location.pathname + location.search) + location.hash;
}

async function request(method, path, { params, body } = {}) {
  let resp;
  try {
    resp = await fetch('/api/v1' + path + qs(params), {
      method,
      headers: body !== undefined ? { 'Content-Type': 'application/json' } : {},
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new ApiError(0, 'Cannot reach the App server');
  }
  if (resp.status === 401 && !path.startsWith('/auth/')) {
    redirectToLogin();
    throw new ApiError(401, 'Sign in required');
  }
  const text = await resp.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { /* non-JSON body */ }
  if (!resp.ok) throw new ApiError(resp.status, (data && data.error) || `Request failed (HTTP ${resp.status})`);
  return data;
}

export const api = {
  get: (path, params) => request('GET', path, { params }),
  post: (path, body = {}) => request('POST', path, { body }),
  put: (path, body = {}) => request('PUT', path, { body }),
  patch: (path, body = {}) => request('PATCH', path, { body }),
  del: (path, params) => request('DELETE', path, { params }),

  // Downloads a server-generated file (export).
  async download(path, params) {
    const resp = await fetch('/api/v1' + path + qs(params));
    if (resp.status === 401) return redirectToLogin();
    if (!resp.ok) {
      let msg = `Export failed (HTTP ${resp.status})`;
      try { msg = (await resp.json()).error || msg; } catch { /* keep default */ }
      throw new ApiError(resp.status, msg);
    }
    const name = /filename="([^"]+)"/.exec(resp.headers.get('Content-Disposition') || '')?.[1] || 'app-export';
    const url = URL.createObjectURL(await resp.blob());
    const a = Object.assign(document.createElement('a'), { href: url, download: name });
    document.body.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  },
};
```

### `web/static/js/state.js`

```js
// Shared app state: signed-in user, events, localStorage and the hash router.

const read = (k, d) => { try { return JSON.parse(localStorage.getItem(k)) ?? d; } catch { return d; } };
export const save = (k, v) => { try { localStorage.setItem(k, JSON.stringify(v)); } catch { /* private mode */ } };
export const load = read;

export const store = {
  version: '',
  commit: '',
  userId: '',
  user: '', // username
  displayName: '',
  role: '', // admin | operator
  authEnabled: true,
};

// setUser stores the signed-in identity ({id, username, display_name, role}).
export function setUser(u) {
  store.userId = u.id || '';
  store.user = u.username || '';
  store.displayName = u.display_name || u.username || '';
  store.role = u.role || '';
  emit('user');
}

const listeners = {};
export function on(evt, fn) {
  (listeners[evt] ||= new Set()).add(fn);
  return () => listeners[evt].delete(fn);
}
export function emit(evt, data) {
  listeners[evt]?.forEach((fn) => fn(data));
}

// ── Router: #/<view>?<params> ──
export function getRoute() {
  const raw = location.hash.replace(/^#\/?/, '');
  const [view, query = ''] = raw.split('?');
  return { view: view || 'dashboard', params: Object.fromEntries(new URLSearchParams(query)) };
}

export function href(view, params = {}) {
  const q = new URLSearchParams(Object.entries(params).filter(([, v]) => v !== '' && v != null)).toString();
  return `#/${view}${q ? '?' + q : ''}`;
}

export function setRoute(view, params = {}, { replace = false } = {}) {
  const hash = href(view, params);
  if (hash === location.hash) return;
  if (replace) {
    history.replaceState(null, '', hash);
    emit('route');
  } else {
    location.hash = hash;
  }
}

export function patchRoute(changes, opts) {
  const r = getRoute();
  setRoute(r.view, { ...r.params, ...changes }, opts);
}
```

### `web/static/js/ui.js`

```js
// DOM and formatting helpers shared by every view.

const SVG_NS = 'http://www.w3.org/2000/svg';
const PROPS = new Set(['value', 'checked', 'selected', 'disabled', 'hidden']);

// h('div', {class: 'x', onclick: fn}, child, [children], 'text')
export function h(tag, attrs, ...children) {
  const el = document.createElement(tag);
  setAttrs(el, attrs);
  appendAll(el, children);
  return el;
}

export function s(tag, attrs, ...children) {
  const el = document.createElementNS(SVG_NS, tag);
  for (const [k, v] of Object.entries(attrs || {})) if (v != null) el.setAttribute(k, v);
  appendAll(el, children);
  return el;
}

function setAttrs(el, attrs) {
  if (!attrs) return;
  for (const [k, v] of Object.entries(attrs)) {
    if (v == null || v === false) continue;
    if (k === 'class') el.className = v;
    else if (k === 'style' && typeof v === 'object') Object.assign(el.style, v);
    else if (k === 'dataset') Object.assign(el.dataset, v);
    else if (k.startsWith('on') && typeof v === 'function') el.addEventListener(k.slice(2), v);
    else if (PROPS.has(k)) el[k] = v;
    else el.setAttribute(k, v === true ? '' : v);
  }
}

function appendAll(el, children) {
  for (const c of children.flat(Infinity)) {
    if (c == null || c === false) continue;
    el.append(c instanceof Node ? c : String(c));
  }
}

export function icon(name, cls = '') {
  const svg = document.createElementNS(SVG_NS, 'svg');
  svg.setAttribute('class', ('icon ' + cls).trim());
  svg.setAttribute('aria-hidden', 'true');
  const use = document.createElementNS(SVG_NS, 'use');
  use.setAttribute('href', '#i-' + name);
  svg.append(use);
  return svg;
}

// ── Time ──
export function fmtAgo(ms) {
  const s = Math.max(0, (Date.now() - ms) / 1000);
  if (s < 60) return 'just now';
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
}

// ── Numbers ──
export const fmtNum = (n) => (n || 0).toLocaleString('en-US');
export function fmtCompact(n) {
  n = n || 0;
  if (n < 1000) return String(n);
  if (n < 1e6) return (n / 1e3).toFixed(n < 1e4 ? 1 : 0).replace(/\.0$/, '') + 'k';
  return (n / 1e6).toFixed(1).replace(/\.0$/, '') + 'M';
}
export function fmtBytes(b) {
  if (!b) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.min(units.length - 1, Math.floor(Math.log(b) / Math.log(1024)));
  return `${parseFloat((b / 1024 ** i).toFixed(1))} ${units[i]}`;
}

// ── Feedback ──
export function toast(msg, kind = 'ok') {
  const t = h('div', { class: `toast ${kind}` }, icon(kind === 'error' ? 'alert' : 'check'), h('span', null, msg));
  document.getElementById('toasts').append(t);
  setTimeout(() => {
    t.classList.add('out');
    setTimeout(() => t.remove(), 250);
  }, kind === 'error' ? 6000 : 3500);
}

export async function copy(text) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    // Clipboard API needs a secure context; fall back for plain-HTTP reverse proxies.
    const ta = h('textarea', { style: { position: 'fixed', opacity: '0' } });
    ta.value = text;
    document.body.append(ta);
    ta.select();
    document.execCommand('copy');
    ta.remove();
  }
  toast('Copied to clipboard');
}

export function confirmDialog({ title, body, confirmText = 'Confirm', danger = false, typeToConfirm = '' }) {
  return new Promise((resolve) => {
    const input = typeToConfirm ? h('input', { class: 'input', autocomplete: 'off', spellcheck: 'false' }) : null;
    const ok = h('button', { class: `btn ${danger ? 'btn-danger solid' : 'btn-primary'}`, disabled: !!typeToConfirm }, confirmText);
    const close = (v) => {
      overlay.remove();
      document.removeEventListener('keydown', onKey);
      resolve(v);
    };
    const onKey = (e) => { if (e.key === 'Escape') close(false); };
    input?.addEventListener('input', () => { ok.disabled = input.value !== typeToConfirm; });
    input?.addEventListener('keydown', (e) => { if (e.key === 'Enter' && !ok.disabled) close(true); });
    ok.addEventListener('click', () => close(true));
    const overlay = h('div', { class: 'overlay', onclick: (e) => { if (e.target === overlay) close(false); } },
      h('div', { class: 'dialog', role: 'dialog', 'aria-modal': 'true' },
        h('h3', null, title),
        h('p', { class: 'dialog-body' }, body),
        input && h('label', { class: 'field' }, h('span', null, `Type ${typeToConfirm} to confirm`), input),
        h('div', { class: 'dialog-actions' }, h('button', { class: 'btn', onclick: () => close(false) }, 'Cancel'), ok)));
    document.body.append(overlay);
    document.addEventListener('keydown', onKey);
    (input || ok).focus();
  });
}

// Small dropdown anchored to a button; opens upwards when there is no room below.
export function menu(anchor, items, { up = false } = {}) {
  document.querySelectorAll('.menu').forEach((m) => m.remove());
  const r = anchor.getBoundingClientRect();
  const m = h('div', { class: 'menu', role: 'menu' },
    items.map((it) => it.label
      ? h('div', { class: 'menu-label' }, it.label)
      : h('button', { type: 'button', class: it.checked ? 'checked' : null, onclick: () => { m.remove(); it.onClick(); } }, it.icon && icon(it.icon), it.text)));
  if (up) m.style.minWidth = `${r.width}px`;
  document.body.append(m);
  const left = up ? r.left : Math.min(r.right - m.offsetWidth, window.innerWidth - m.offsetWidth - 8);
  let top = r.bottom + 6;
  if (up || top + m.offsetHeight > window.innerHeight - 8) top = Math.max(8, r.top - m.offsetHeight - 6);
  Object.assign(m.style, { top: `${top}px`, left: `${Math.max(8, left)}px` });
  setTimeout(() => {
    const off = (e) => {
      if (!m.contains(e.target)) { m.remove(); document.removeEventListener('mousedown', off); }
    };
    document.addEventListener('mousedown', off);
  });
}

export function emptyState(iconName, title, body, action) {
  return h('div', { class: 'empty' }, icon(iconName), h('h3', null, title), body && h('p', null, body), action);
}

export function skeletonRows(n = 10) {
  return Array.from({ length: n }, (_, i) => h('div', { class: 'skel skel-row', style: { width: `${55 + ((i * 37) % 40)}%` } }));
}

// Dialog with a small form. onSubmit(values) may throw to show the error and
// keep the dialog open; its result resolves the promise (false on cancel).
// fields: [{name, label, type, value, options: [[value, label]], hint, required, autocomplete, placeholder}]
export function formDialog({ title, body, fields = [], submitText = 'Save', danger = false, wide = false, cancel = true, onSubmit = async () => true }) {
  return new Promise((resolve) => {
    const inputs = {};
    const err = h('p', { class: 'form-error' });
    const ok = h('button', { class: `btn ${danger ? 'btn-danger solid' : 'btn-primary'}`, type: 'submit' }, submitText);
    const rows = fields.map((f) => {
      if (f.node) return h('div', { class: 'field' }, f.label ? h('span', null, f.label) : null, f.node);
      const input = f.type === 'select'
        ? h('select', { class: 'select wide', name: f.name }, f.options.map(([v, l]) => h('option', { value: v }, l)))
        : h('input', {
          class: 'input', name: f.name, type: f.type || 'text', autocomplete: f.autocomplete || 'off', spellcheck: 'false',
          placeholder: f.placeholder || '', required: f.required !== false,
        });
      if (f.value != null) input.value = f.value;
      inputs[f.name] = input;
      return h('label', { class: 'field' }, h('span', null, f.label), input, f.hint ? h('small', { class: 'muted' }, f.hint) : null);
    });
    const close = (v) => {
      overlay.remove();
      document.removeEventListener('keydown', onKey);
      resolve(v);
    };
    const onKey = (e) => { if (e.key === 'Escape') close(false); };
    const form = h('form', { class: `dialog${wide ? ' wide' : ''}`, role: 'dialog', 'aria-modal': 'true' },
      h('h3', null, title), body ? h('p', { class: 'dialog-body' }, body) : null, rows, err,
      h('div', { class: 'dialog-actions' }, cancel ? h('button', { class: 'btn', type: 'button', onclick: () => close(false) }, 'Cancel') : null, ok));
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      err.textContent = '';
      ok.disabled = true;
      try {
        const res = await onSubmit(Object.fromEntries(Object.entries(inputs).map(([k, el]) => [k, el.value])));
        close(res ?? true);
      } catch (e2) {
        err.textContent = e2.message;
      } finally {
        ok.disabled = false;
      }
    });
    const overlay = h('div', { class: 'overlay', onmousedown: (e) => { if (e.target === overlay) close(false); } }, form);
    document.body.append(overlay);
    document.addEventListener('keydown', onKey);
    (Object.values(inputs)[0] || ok).focus();
  });
}

// Password policy, mirrored from the server (auth.checkPassword).
const PW_RULES = [
  ['8+ characters', (p) => [...p].length >= 8],
  ['lowercase', (p) => /\p{Ll}/u.test(p)],
  ['uppercase', (p) => /\p{Lu}/u.test(p)],
  ['number', (p) => /\p{Nd}/u.test(p)],
  ['symbol', (p) => /[^\p{L}\p{Nd}\s]/u.test(p)],
];
export const PASSWORD_HINT = 'At least 8 characters with a-z, A-Z, 0-9 and a symbol, e.g. Qawsed#1477';

// Returns what a password is missing, or '' when it follows the policy.
export function passwordProblem(p) {
  if (new TextEncoder().encode(p).length > 72) return 'Password must be at most 72 characters.';
  const missing = PW_RULES.filter(([, ok]) => !ok(p)).map(([name]) => name);
  return missing.length ? `Password needs: ${missing.join(', ')} (e.g. Qawsed#1477).` : '';
}

// Checklist under a password input that ticks the rules off while typing.
export function passwordRules(input) {
  const items = PW_RULES.map(([name]) => h('li', null, icon('checkmark'), name));
  const sync = () => PW_RULES.forEach(([, ok], i) => items[i].classList.toggle('ok', ok(input.value)));
  input.addEventListener('input', sync);
  sync();
  return h('ul', { class: 'pw-rules', 'aria-label': 'Password rules' }, items);
}

// Readable random password (no look-alike characters) that follows the policy.
export function randomPassword(n = 14) {
  const sets = ['abcdefghjkmnpqrstuvwxyz', 'ABCDEFGHJKMNPQRSTUVWXYZ', '23456789', '#@$%&*!?'];
  const all = sets.join('');
  const rnd = (max) => crypto.getRandomValues(new Uint32Array(1))[0] % max;
  const out = sets.map((set) => set[rnd(set.length)]); // one of each class
  while (out.length < n) out.push(all[rnd(all.length)]);
  for (let i = out.length - 1; i > 0; i--) { // shuffle so the classes are not in a fixed order
    const j = rnd(i + 1);
    [out[i], out[j]] = [out[j], out[i]];
  }
  return out.join('');
}

export function debounce(fn, ms) {
  let t;
  return (...args) => {
    clearTimeout(t);
    t = setTimeout(() => fn(...args), ms);
  };
}
```

### `web/static/js/palette.js`

```js
// Ctrl+K command palette: jump to any page or run an action.
// items: [{group, label, hint, icon, keywords, run}], built on open.
import { h, icon } from './ui.js';

let openEl = null;

// Scores how well q matches an item: text in the label beats text in its
// keywords, which beats the letters of q in order inside the label ("dsb").
function score(q, it) {
  if (!q) return 1;
  const label = it.label.toLowerCase();
  const i = label.indexOf(q);
  if (i >= 0) return 1000 - i - label.length / 100;
  const all = `${label} ${it.keywords || ''} ${it.hint || ''}`.toLowerCase();
  const words = q.split(/\s+/);
  if (words.every((w) => all.includes(w))) return 500 - label.length / 100;
  let pos = 0;
  let gaps = 0;
  for (const ch of q.replace(/\s+/g, '')) {
    const j = label.indexOf(ch, pos);
    if (j < 0) return 0;
    gaps += j - pos;
    pos = j + 1;
  }
  return gaps <= label.length / 2 ? 100 - gaps : 0;
}

export function openPalette(build) {
  if (openEl) return;
  const input = h('input', { class: 'palette-input', type: 'text', autocomplete: 'off', spellcheck: 'false', placeholder: 'Go to a page or run an action', 'aria-label': 'Search menu' });
  const list = h('div', { class: 'palette-list', role: 'listbox' });
  const foot = h('div', { class: 'palette-foot' },
    h('span', null, h('kbd', null, '↑↓'), ' move'), h('span', null, h('kbd', null, 'Enter'), ' open'), h('span', null, h('kbd', null, 'Esc'), ' close'));
  const box = h('div', { class: 'palette', role: 'dialog', 'aria-modal': 'true', 'aria-label': 'Command menu' },
    h('div', { class: 'palette-search' }, icon('search'), input), list, foot);
  const overlay = h('div', { class: 'overlay palette-overlay', onmousedown: (e) => { if (e.target === overlay) close(); } }, box);
  let shown = [];
  let sel = 0;

  function close() {
    overlay.remove();
    openEl = null;
  }

  function render() {
    const q = input.value.trim().toLowerCase();
    const items = build(input.value.trim());
    shown = items
      .map((it) => ({ it, s: it.always ? 2000 : score(q, it) }))
      .filter((x) => x.s > 0)
      .sort((a, b) => (q ? b.s - a.s : 0))
      .slice(0, 60)
      .map((x) => x.it);
    sel = Math.min(sel, Math.max(0, shown.length - 1));
    if (!shown.length) {
      list.replaceChildren(h('div', { class: 'palette-empty' }, 'Nothing matches'));
      return;
    }
    const rows = [];
    let group = null;
    shown.forEach((it, i) => {
      if (!q && it.group !== group) {
        group = it.group;
        rows.push(h('div', { class: 'palette-group' }, group));
      }
      rows.push(h('button', {
        type: 'button', class: `palette-item${i === sel ? ' sel' : ''}`, role: 'option',
        onmousemove: () => { if (sel !== i) { sel = i; mark(); } },
        onclick: () => run(i),
      }, icon(it.icon || 'arrow-right'), h('span', { class: 'label' }, it.label), it.hint ? h('span', { class: 'hint' }, it.hint) : null));
    });
    list.replaceChildren(...rows);
  }

  function mark() {
    list.querySelectorAll('.palette-item').forEach((b, i) => b.classList.toggle('sel', i === sel));
    list.querySelectorAll('.palette-item')[sel]?.scrollIntoView({ block: 'nearest' });
  }

  function run(i) {
    const it = shown[i];
    if (!it) return;
    close();
    it.run();
  }

  input.addEventListener('input', () => { sel = 0; render(); });
  input.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); sel = Math.min(sel + 1, shown.length - 1); mark(); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); sel = Math.max(sel - 1, 0); mark(); }
    else if (e.key === 'Enter') { e.preventDefault(); run(sel); }
    else if (e.key === 'Escape') { e.preventDefault(); close(); }
  });

  document.body.append(overlay);
  openEl = overlay;
  render();
  input.focus();
}
```

### `web/static/js/app.js`

```js
// App shell: router, grouped sidebar, command palette (Ctrl+K), topbar.
import { api, redirectToLogin } from './api.js';
import { h, icon, menu, emptyState, toast } from './ui.js';
import { store, on, save, load, getRoute, setRoute, setUser } from './state.js';
import { openPalette } from './palette.js';
import * as dashboard from './views/dashboard.js';
import * as profile from './views/profile.js';
import * as users from './views/users.js';
import * as about from './views/about.js';

const APP = 'App';
const VIEWS = { dashboard, profile, users, about };

// Sidebar menu, top to bottom. Groups without a title render as plain links.
// auth: only with authentication enabled; admin: only for administrators.
const NAV = [
  { id: 'main', links: [{ view: 'dashboard', label: 'Dashboard', icon: 'overview', keywords: 'home overview' }] },
  // { id: 'data', title: 'Data', links: [{ view: 'items', label: 'Items', icon: 'box', keywords: '...' }] },
  {
    id: 'account', title: 'Account', auth: true, links: [
      { view: 'profile', label: 'Profile', icon: 'user', keywords: 'account password 2fa qr recovery' },
      { view: 'users', label: 'Users', icon: 'users', keywords: 'accounts roles operators admin', admin: true },
    ],
  },
  { id: 'about', links: [{ view: 'about', label: 'About', icon: 'info', keywords: 'author credits stack license version help' }] },
];

const $ = (id) => document.getElementById(id);
const rootEl = document.documentElement;
const viewRoot = $('view');
const collapsed = new Set(load('app_groups', []));
const isAdmin = () => store.authEnabled && store.role === 'admin';
const visible = (item) => (!item.auth || store.authEnabled) && (!item.admin || isAdmin());
const pages = () => NAV.filter(visible).flatMap((g) => g.links.filter(visible));
let current = null;

// ── Routing ──
function renderRoute() {
  const r = getRoute();
  const page = pages().find((l) => l.view === r.view);
  if (!VIEWS[r.view] || !page) return setRoute('dashboard', {}, { replace: true });
  if (current?.view !== r.view) {
    current?.inst.destroy?.();
    viewRoot.replaceChildren();
    viewRoot.scrollTop = 0;
    current = { view: r.view, inst: VIEWS[r.view].mount(viewRoot) };
  }
  current.inst.update?.(r.params);
  document.title = `${page.label} - ${APP}`;
  rootEl.classList.remove('sidebar-open');
  $('crumbs').replaceChildren(h('b', null, page.label));
  renderSidebar(r);
}

// ── Sidebar ──
function sideLink(to, label, ic, active, extra = []) {
  return h('a', { class: `side-link${active ? ' active' : ''}`, href: to, title: label }, icon(ic), h('span', { class: 'name' }, label), ...extra);
}

function sideGroup(id, title, links) {
  const el = h('section', { class: `side-group${collapsed.has(id) ? ' collapsed' : ''}` });
  if (title) {
    el.append(h('button', {
      class: 'side-group-title', type: 'button',
      onclick: () => {
        if (collapsed.has(id)) collapsed.delete(id);
        else collapsed.add(id);
        save('app_groups', [...collapsed]);
        el.classList.toggle('collapsed');
      },
    }, title, icon('chevron-down')));
  }
  el.append(h('div', { class: 'side-links' }, links));
  return el;
}

function renderSidebar(r = getRoute()) {
  $('side-nav').replaceChildren(...NAV.filter(visible).map((g) => {
    const links = g.links.filter(visible).map((l) => sideLink(`#/${l.view}`, l.label, l.icon, r.view === l.view));
    return links.length ? sideGroup(g.id, g.title || '', links) : null;
  }).filter(Boolean));
}

// ── Command palette: pages from NAV, then actions ──
function paletteItems() {
  const items = pages().map((l) => ({ group: 'Pages', label: l.label, icon: l.icon, keywords: l.keywords || '', run: () => setRoute(l.view) }));
  items.push(
    { group: 'Actions', label: 'Toggle dark mode', icon: rootEl.dataset.theme === 'dark' ? 'sun' : 'moon', keywords: 'theme light', run: toggleTheme },
    { group: 'Actions', label: 'Refresh data', icon: 'refresh', keywords: 'reload', run: refreshAll },
  );
  if (store.authEnabled) items.push({ group: 'Actions', label: 'Sign out', icon: 'logout', keywords: 'logout', run: signOut });
  return items;
}

const showPalette = () => openPalette(paletteItems);
$('palette').addEventListener('click', showPalette);
$('search-kbd').textContent = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘K' : 'Ctrl K';

// Ctrl+K opens the menu; "/" jumps to the search box of the current page.
document.addEventListener('keydown', (e) => {
  const typing = /INPUT|TEXTAREA|SELECT/.test(document.activeElement?.tagName || '');
  if (e.key === 'k' && (e.metaKey || e.ctrlKey)) {
    e.preventDefault();
    showPalette();
  } else if (e.key === '/' && !typing) {
    e.preventDefault();
    const box = document.querySelector('[data-search]');
    if (box) { box.focus(); box.select(); } else showPalette();
  }
});

// ── Topbar ──
function toggleTheme() {
  const t = rootEl.dataset.theme === 'dark' ? 'light' : 'dark';
  rootEl.dataset.theme = t;
  try { localStorage.setItem('app_theme', t); } catch { /* private mode */ }
}

function refreshAll() {
  current?.inst.refresh?.();
}

$('toggle-sidebar').addEventListener('click', () => {
  if (matchMedia('(max-width: 860px)').matches) {
    rootEl.classList.toggle('sidebar-open');
    return;
  }
  save('app_sidebar', rootEl.classList.toggle('sidebar-collapsed') ? 'collapsed' : '');
});
document.querySelector('.main').addEventListener('click', (e) => {
  if (!e.target.closest('#toggle-sidebar')) rootEl.classList.remove('sidebar-open');
});
$('theme').addEventListener('click', toggleTheme);
$('refresh').addEventListener('click', refreshAll);

async function signOut() {
  try { await api.post('/auth/logout'); } catch { /* already gone */ }
  location.href = '/login';
}

function renderAvatar() {
  $('avatar').textContent = (store.displayName || store.user || '?').charAt(0).toUpperCase();
  $('avatar').title = store.authEnabled ? `${store.displayName} (${store.role})` : 'Account';
}

$('avatar').addEventListener('click', () => menu($('avatar'), store.authEnabled
  ? [
    { label: `${store.displayName} · ${store.role === 'admin' ? 'Administrator' : 'Operator'}` },
    { icon: 'user', text: 'Profile', onClick: () => setRoute('profile') },
    ...(isAdmin() ? [{ icon: 'users', text: 'Users', onClick: () => setRoute('users') }] : []),
    { icon: 'info', text: 'About', onClick: () => setRoute('about') },
    { icon: 'logout', text: 'Sign out', onClick: signOut },
  ]
  : [{ label: 'Authentication is disabled' }, { icon: 'info', text: 'About', onClick: () => setRoute('about') }]));

// ── Start ──
async function start() {
  let st;
  try {
    st = await api.get('/auth/status');
  } catch (e) {
    viewRoot.replaceChildren(emptyState('alert', `Cannot reach ${APP}`, e.message,
      h('button', { class: 'btn', onclick: () => location.reload() }, 'Retry')));
    return;
  }
  if (st.auth_enabled && !st.authenticated) return redirectToLogin();
  store.authEnabled = st.auth_enabled;
  setUser(st.user || { id: '', username: st.username, display_name: st.username, role: 'admin' });
  renderAvatar();

  const cfg = await api.get('/config').catch(() => null);
  if (cfg) {
    store.version = cfg.version;
    store.commit = cfg.commit;
    $('brand-ver').textContent = [...new Set([cfg.version, cfg.commit].filter(Boolean))].join(' · ');
  }

  on('user', () => { renderAvatar(); renderSidebar(); });
  on('route', renderRoute);
  window.addEventListener('hashchange', renderRoute);
  renderRoute();

  // An administrator without a recovery question can only be rescued by another admin.
  if (isAdmin()) {
    api.get('/profile').then((p) => {
      if (!p.user.has_recovery) toast('Set a recovery question in your profile, so a lost password or phone can be recovered', 'error');
    }).catch(() => {});
  }
}

start();
```

### `web/static/js/views/items.js`

```js
// Items: one sentence on what this page shows and why.
import { api } from '../api.js';
import { h, icon, emptyState, toast } from '../ui.js';

export function mount(root) {
  let alive = true;
  const body = h('div', { class: 'grid grid-2e' });
  root.append(h('div', { class: 'page' },
    h('div', { class: 'page-head' },
      h('div', null, h('h1', null, 'Items'), h('p', null, 'One line on what this page is for')),
      h('div', { class: 'page-actions' },
        h('button', { class: 'btn btn-primary', type: 'button', onclick: () => toast('Not implemented yet') }, icon('plus'), 'Add item'))),
    body));

  const card = (title, sub, ...content) => h('div', { class: 'card' },
    h('div', { class: 'card-head' }, h('div', null, h('h2', null, title), h('p', { class: 'card-sub' }, sub))), ...content);

  async function load() {
    body.replaceChildren(...[0, 1].map(() => h('div', { class: 'card' }, h('div', { class: 'skel', style: { height: '120px' } }))));
    let list;
    try {
      list = await api.get('/items');
    } catch (e) {
      if (alive) body.replaceChildren(emptyState('alert', 'Could not load items', e.message));
      return;
    }
    if (!alive) return;
    if (!list?.length) return body.replaceChildren(emptyState('inbox', 'No items yet', 'Items show up here once they are created.'));
    body.replaceChildren(card('Items', `${list.length} stored`,
      h('div', { class: 'table-wrap' }, h('table', { class: 'table' },
        h('thead', null, h('tr', null, h('th', null, 'Name'), h('th', { class: 'num' }, 'Count'))),
        h('tbody', null, list.map((it) => h('tr', null, h('td', { class: 'strong' }, it.name), h('td', { class: 'num' }, it.count))))))));
  }

  load();
  return {
    update() {}, // URL params changed
    refresh: load,
    destroy() { alive = false; },
  };
}
```

### `web/static/js/views/profile.js`

```js
// The signed-in user's own account: name, password, authenticator, recovery.
import { api } from '../api.js';
import { h, icon, toast, copy, formDialog, emptyState, fmtAgo, passwordProblem, passwordRules } from '../ui.js';
import { store, setUser } from '../state.js';

export function mount(root) {
  let alive = true;
  let hideTimer = null;
  const el = h('div', { class: 'page' });
  root.append(el);
  const head = h('div', { class: 'page-head' },
    h('div', null, h('h1', null, 'Profile'), h('p', null, 'Your account, password and two-factor sign-in')));

  const card = (title, sub, ...content) => h('div', { class: 'card' },
    h('div', { class: 'card-head' }, h('div', null, h('h2', null, title), h('p', { class: 'card-sub' }, sub))), ...content);
  const input = (attrs) => h('input', { class: 'input', spellcheck: 'false', ...attrs });
  const labeled = (label, control, cls = '') => h('label', { class: `field ${cls}` }, h('span', null, label), control);
  const staticField = (label, value) => h('div', { class: 'field' }, h('span', null, label), h('div', { class: 'static' }, value));
  const actions = (...btns) => h('div', { class: 'form-actions' }, ...btns);

  async function save(btn, fn) {
    btn.disabled = true;
    try { await fn(); } catch (e) { toast(e.message, 'error'); } finally { btn.disabled = false; }
  }

  async function load() {
    if (!store.authEnabled) {
      el.replaceChildren(head, emptyState('lock', 'Authentication is disabled', 'Set APP_AUTH_ENABLED=true to use accounts and two-factor sign-in.'));
      return;
    }
    let data;
    try {
      data = await api.get('/profile');
    } catch (e) {
      if (alive) el.replaceChildren(head, emptyState('alert', 'Could not load your profile', e.message));
      return;
    }
    if (alive) render(data.user, data.questions || []);
  }

  function accountCard(u) {
    const name = input({ value: u.display_name || '', placeholder: u.username, autocomplete: 'name', maxlength: 64 });
    const uname = input({ value: u.username, autocomplete: 'username', required: true });
    const btn = h('button', { class: 'btn btn-primary', type: 'submit' }, 'Save');
    const form = h('form', null,
      h('div', { class: 'form-grid' },
        labeled('Display name', name), labeled('Username', uname),
        staticField('Role', u.role === 'admin' ? 'Administrator' : 'Operator'),
        staticField('Last sign-in', u.last_login_at ? fmtAgo(Date.parse(u.last_login_at)) : '-')),
      actions(btn));
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      save(btn, async () => {
        const nu = await api.patch('/profile', { display_name: name.value, username: uname.value });
        setUser(nu);
        toast('Profile saved');
      });
    });
    return card('Account', 'How you appear and sign in', form);
  }

  function passwordCard() {
    const cur = input({ type: 'password', autocomplete: 'current-password', required: true });
    const p1 = input({ type: 'password', autocomplete: 'new-password', minlength: 8, required: true, placeholder: 'e.g. Qawsed#1477' });
    const p2 = input({ type: 'password', autocomplete: 'new-password', minlength: 8, required: true });
    const btn = h('button', { class: 'btn btn-primary', type: 'submit' }, 'Change password');
    const form = h('form', null,
      h('div', { class: 'form-grid' }, labeled('Current password', cur, 'full'), labeled('New password', p1), labeled('Confirm new password', p2),
        h('div', { class: 'full' }, passwordRules(p1))),
      actions(btn));
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      const problem = passwordProblem(p1.value);
      if (problem) return toast(problem, 'error');
      if (p1.value !== p2.value) return toast('The new passwords do not match', 'error');
      save(btn, async () => {
        await api.post('/profile/password', { current: cur.value, password: p1.value });
        form.reset();
        toast('Password changed, your other sessions were signed out');
      });
    });
    return card('Password', 'Changing it signs out your other sessions', form);
  }

  function twoFactorCard(u) {
    const body = h('div');
    const showBtn = h('button', { class: 'btn', type: 'button' }, icon('shield'), 'Show QR code');
    const hide = () => {
      clearTimeout(hideTimer);
      body.replaceChildren(h('div', { class: 'tf-row' },
        u.has_2fa ? h('span', { class: 'badge ok' }, 'Enabled') : h('span', { class: 'badge err' }, 'Not set up'),
        h('span', { class: 'muted' }, 'Codes come from your authenticator app'),
        h('span', { class: 'spacer' }), u.has_2fa ? showBtn : null));
    };
    const show = (res) => {
      body.replaceChildren(
        h('div', { class: 'qr' },
          h('img', { src: res.qr_data_url, alt: 'QR code for your authenticator app' }),
          h('div', null,
            h('div', { class: 'muted' }, 'Scan it on your new phone, or enter the key:'),
            h('code', null, res.secret),
            h('span', { class: 'input-group' },
              h('button', { class: 'btn btn-sm', type: 'button', onclick: () => copy(res.secret) }, icon('copy'), 'Copy key'),
              h('button', { class: 'btn btn-sm', type: 'button', onclick: hide }, icon('x'), 'Hide')))),
        h('p', { class: 'note', style: { marginTop: '10px' } }, icon('help'),
          'Both phones show the same codes. Remove App from the old phone once the new one works. Hidden again in 2 minutes.'));
      clearTimeout(hideTimer);
      hideTimer = setTimeout(hide, 120e3);
    };
    showBtn.addEventListener('click', async () => {
      const res = await formDialog({
        title: 'Show your authenticator key',
        body: 'Enter your password to show the QR code for moving two-factor sign-in to another phone.',
        fields: [{ name: 'password', label: 'Password', type: 'password', autocomplete: 'current-password' }],
        submitText: 'Show QR code',
        onSubmit: (v) => api.post('/profile/2fa', v),
      });
      if (res && res.secret) show(res);
    });
    hide();
    return card('Two-factor sign-in', 'Show the QR code again to move it to a new phone', body);
  }

  function recoveryCard(u, questions) {
    if (u.role !== 'admin') {
      return card('Account recovery', 'Lost your password or phone?',
        h('p', { class: 'note' }, icon('users'), 'An administrator can reset your password or two-factor sign-in from the Users page.'));
    }
    const q = h('select', { class: 'select wide', required: true },
      h('option', { value: '' }, 'Choose a question'), questions.map((x) => h('option', { value: x.id }, x.text)));
    q.value = u.recovery_question || '';
    const ans = input({ autocomplete: 'off', required: true, placeholder: u.has_recovery ? 'Type the answer again to change it' : 'Not case sensitive' });
    const pw = input({ type: 'password', autocomplete: 'current-password', required: true });
    const btn = h('button', { class: 'btn btn-primary', type: 'submit' }, 'Save recovery question');
    const form = h('form', null,
      u.has_recovery
        ? h('p', { class: 'note' }, icon('check'), 'Set. On the sign-in page, "Forgot your password" asks it plus your 2FA code or your password.')
        : h('p', { class: 'note warn' }, icon('alert'), 'Not set. Without it only another administrator can reset a lost password or phone.'),
      h('div', { class: 'form-grid mt-sm' }, labeled('Question', q, 'full'), labeled('Answer', ans), labeled('Current password', pw)),
      actions(btn));
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      save(btn, async () => {
        await api.put('/profile/recovery', { question: q.value, answer: ans.value, password: pw.value });
        toast('Recovery question saved');
        load();
      });
    });
    return card('Account recovery', 'Recover a lost password or phone yourself', form);
  }

  function render(u, questions) {
    el.replaceChildren(head,
      h('div', { class: 'grid grid-2e' }, accountCard(u), passwordCard()),
      h('div', { class: 'grid grid-2e mt' }, twoFactorCard(u), recoveryCard(u, questions)));
  }

  load();
  return { refresh: load, destroy() { alive = false; clearTimeout(hideTimer); } };
}
```

### `web/static/js/views/users.js`

```js
// User management (administrators): accounts, roles, password and 2FA resets.
import { api } from '../api.js';
import { h, icon, fmtAgo, toast, menu, confirmDialog, formDialog, randomPassword, copy, emptyState, passwordProblem, PASSWORD_HINT } from '../ui.js';
import { store, setUser } from '../state.js';

const ROLES = [['operator', 'Operator: everything except user management'], ['admin', 'Administrator: also manages users']];
const roleBadge = (r) => h('span', { class: `badge${r === 'admin' ? ' role-admin' : ''}` }, r === 'admin' ? 'Admin' : 'Operator');

// Shows sign-in details to hand over to a user.
function shareDetails(heading, username, password) {
  const text = `App sign-in\nAddress: ${location.origin}/login\nUsername: ${username}\nTemporary password: ${password}`;
  return formDialog({
    title: heading,
    body: 'Share these privately. At the first sign-in the user chooses a new password and sets up two-factor sign-in.',
    fields: [{
      node: h('div', { class: 'cred' },
        h('div', null, h('span', { class: 'muted' }, 'Username  '), h('b', null, username)),
        h('div', null, h('span', { class: 'muted' }, 'Password  '), h('code', null, password)),
        h('div', null, h('button', { class: 'btn btn-sm', type: 'button', onclick: () => copy(text) }, icon('copy'), 'Copy details'))),
    }],
    submitText: 'Done',
    cancel: false,
  });
}

export function mount(root) {
  let alive = true;
  const box = h('div', { class: 'card table-card' });
  root.append(h('div', { class: 'page' },
    h('div', { class: 'page-head' },
      h('div', null, h('h1', null, 'Users'), h('p', null, 'Administrators manage accounts, operators use everything else')),
      h('div', { class: 'page-actions' }, h('button', { class: 'btn btn-primary', type: 'button', onclick: addUser }, icon('plus'), 'Add user'))),
    box,
    h('p', { class: 'page-foot' }, icon('help'),
      'New and reset accounts sign in with a temporary password, then choose their own and set up two-factor sign-in. ',
      'Administrators can also recover themselves on the sign-in page with their recovery question.')));

  async function load() {
    let list;
    try {
      list = await api.get('/users');
    } catch (e) {
      if (alive) box.replaceChildren(emptyState('alert', 'Could not load users', e.message));
      return;
    }
    if (alive) render(list || []);
  }

  // Sign-in state, plus a button for admins still missing a recovery question.
  function status(u, self) {
    const state = !u.has_2fa ? h('span', { class: 'badge' }, '2FA setup pending')
      : u.must_change_password ? h('span', { class: 'badge' }, 'Temporary password')
        : h('span', { class: 'badge ok' }, 'Active');
    const recovery = u.role === 'admin' && !u.has_recovery
      ? h('button', {
        class: 'badge err badge-btn', type: 'button', title: 'Without it only another administrator can reset this account. Click to set one.',
        onclick: () => setRecovery(u, self),
      }, icon('plus'), 'Set recovery question')
      : null;
    return h('div', { class: 'status-cell' }, state, recovery);
  }

  // Recovery questions: another admin's directly, the own one with the password.
  let questions = null;
  async function setRecovery(u, self) {
    try {
      questions ||= await api.get('/auth/questions');
    } catch (e) {
      return toast(e.message, 'error');
    }
    const fields = [
      { name: 'question', label: 'Question', type: 'select', options: [['', 'Choose a question'], ...questions.map((q) => [q.id, q.text])], value: u.recovery_question || '' },
      { name: 'answer', label: 'Answer', placeholder: 'Not case sensitive', hint: self ? '' : `Tell ${u.username} the answer privately. They can change it later in their profile.` },
    ];
    if (self) fields.push({ name: 'password', label: 'Your current password', type: 'password', autocomplete: 'current-password' });
    const ok = await formDialog({
      title: self ? 'Set your recovery question' : `Recovery question of ${u.username}`,
      body: 'On the sign-in page, "Forgot your password" asks this answer plus the 2FA code (to reset the password) or the password (to reset a lost phone).',
      fields, submitText: 'Save',
      onSubmit: (v) => {
        if (!v.question) throw new Error('Choose a question.');
        return self ? api.put('/profile/recovery', v) : api.put(`/users/${u.id}/recovery`, v);
      },
    });
    if (!ok) return;
    toast('Recovery question saved');
    load();
  }

  function render(list) {
    box.replaceChildren(h('div', { class: 'table-wrap' }, h('table', { class: 'table' },
      h('thead', null, h('tr', null,
        h('th', null, 'User'), h('th', null, 'Role'), h('th', null, 'Status'), h('th', null, 'Last sign-in'), h('th', null, 'Created'), h('th'))),
      h('tbody', null, list.map((u) => {
        const self = u.id === store.userId;
        return h('tr', null,
          h('td', null,
            h('div', { class: 'strong' }, u.display_name || u.username, self ? h('span', { class: 'badge', style: { marginLeft: '6px' } }, 'you') : null),
            h('div', { class: 'sub' }, u.username)),
          h('td', null, roleBadge(u.role)),
          h('td', null, status(u, self)),
          h('td', { class: 'muted' }, u.last_login_at ? fmtAgo(Date.parse(u.last_login_at)) : 'Never'),
          h('td', { class: 'muted' }, (u.created_at || '').slice(0, 10)),
          h('td', { class: 'num' }, actions(u, self)));
      })))));
  }

  function actions(u, self) {
    const recovery = u.role === 'admin'
      ? [{ icon: 'help', text: u.has_recovery ? 'Change recovery question' : 'Set recovery question', onClick: () => setRecovery(u, self) }]
      : [];
    const b = h('button', {
      class: 'icon-btn sm', type: 'button', title: 'Actions',
      onclick: () => menu(b, self
        ? [{ icon: 'pencil', text: 'Edit', onClick: () => editUser(u) }, ...recovery, { icon: 'user', text: 'Open my profile', onClick: () => { location.hash = '#/profile'; } }]
        : [
          { icon: 'pencil', text: 'Edit', onClick: () => editUser(u) },
          ...recovery,
          { icon: 'key', text: 'Reset password', onClick: () => resetPassword(u) },
          { icon: 'shield', text: 'Reset 2FA', onClick: () => reset2FA(u) },
          { icon: 'trash', text: 'Delete', onClick: () => remove(u) },
        ]),
    }, icon('more'));
    return b;
  }

  async function addUser() {
    const res = await formDialog({
      title: 'Add user', wide: true, submitText: 'Create user',
      fields: [
        { name: 'username', label: 'Username', placeholder: 'e.g. budi', autocomplete: 'off' },
        { name: 'display_name', label: 'Display name', required: false, placeholder: 'Optional' },
        { name: 'role', label: 'Role', type: 'select', options: ROLES, value: 'operator' },
        { name: 'password', label: 'Temporary password', value: randomPassword(), hint: `${PASSWORD_HINT}. The user replaces it at the first sign-in.` },
      ],
      onSubmit: async (v) => {
        const problem = passwordProblem(v.password);
        if (problem) throw new Error(problem);
        return { user: await api.post('/users', v), password: v.password };
      },
    });
    if (!res) return;
    toast(`User ${res.user.username} created`);
    load();
    shareDetails('User created', res.user.username, res.password);
  }

  async function editUser(u) {
    const res = await formDialog({
      title: `Edit ${u.username}`,
      fields: [
        { name: 'username', label: 'Username', value: u.username },
        { name: 'display_name', label: 'Display name', value: u.display_name, required: false },
        { name: 'role', label: 'Role', type: 'select', options: ROLES, value: u.role },
      ],
      onSubmit: (v) => api.patch(`/users/${u.id}`, v),
    });
    if (!res) return;
    toast('User saved');
    if (res.id === store.userId) {
      setUser(res);
      if (res.role !== 'admin') return void (location.hash = '#/dashboard');
    }
    load();
  }

  async function resetPassword(u) {
    const password = await formDialog({
      title: `Reset the password of ${u.username}?`, danger: true, submitText: 'Reset password',
      body: `${u.username} is signed out everywhere and chooses a new password at the next sign-in.`,
      fields: [{ name: 'password', label: 'Temporary password', value: randomPassword(), hint: PASSWORD_HINT }],
      onSubmit: async (v) => {
        const problem = passwordProblem(v.password);
        if (problem) throw new Error(problem);
        await api.post(`/users/${u.id}/password`, v);
        return v.password;
      },
    });
    if (!password) return;
    load();
    shareDetails('Password reset', u.username, password);
  }

  async function reset2FA(u) {
    const ok = await confirmDialog({
      title: `Reset two-factor sign-in of ${u.username}?`,
      body: 'Use this when they lost or replaced their phone. They are signed out everywhere and scan a new QR code at the next sign-in.',
      confirmText: 'Reset 2FA', danger: true,
    });
    if (!ok) return;
    try {
      await api.post(`/users/${u.id}/2fa/reset`);
      toast(`2FA of ${u.username} reset`);
    } catch (e) {
      toast(e.message, 'error');
    }
    load();
  }

  async function remove(u) {
    const ok = await confirmDialog({
      title: `Delete ${u.username}?`, body: 'The account and its sessions are removed. This cannot be undone.',
      confirmText: 'Delete user', danger: true, typeToConfirm: u.username,
    });
    if (!ok) return;
    try {
      await api.del(`/users/${u.id}`);
      toast(`User ${u.username} deleted`);
    } catch (e) {
      toast(e.message, 'error');
    }
    load();
  }

  load();
  return { refresh: load, destroy() { alive = false; } };
}
```

### `web/static/js/views/about.js`

```js
// About: why the app exists, what it runs on, and who built it.
// Everything shown here lives in the constants below.
import { h, icon } from '../ui.js';
import { store } from '../state.js';

const APP = 'App';
const TAGLINE = 'One line on what the app is';
const DESCRIPTION = 'Two or three sentences: what the app does, for whom, and what makes it different from the usual solution.';
const LICENSE = 'MIT license';

const AUTHOR = {
  name: 'Full Name',
  github: 'username',
  email: 'name@example.com',
};

// Why the app exists: what breaks without it, and what the usual fix costs.
const PROBLEMS = [
  ['First problem', 'One sentence on the situation and why it hurts.'],
  ['Second problem', 'One sentence.'],
  ['The usual answer is heavy', 'One sentence on what the common solution costs.'],
];

const STACK = [
  ['Go', 'One static binary with the web UI embedded.'],
  ['bcrypt + TOTP (RFC 6238)', 'Password hashing, two-factor sign-in with QR enrollment, and HMAC-signed session cookies.'],
  ['Vanilla JS + go:embed', 'The whole dashboard ships inside the binary: no Node build step, no CDN, no external assets.'],
  ['JetBrains Mono Nerd Font', 'Monospace everywhere, so numbers, code and glyphs line up.'],
];

const FEATURES = [
  ['Main feature', 'What it does, in one sentence.'],
  ['Second feature', 'What it does, in one sentence.'],
  ['Accounts and 2FA', 'Admin and operator roles, recovery questions, and per-user authenticator enrollment.'],
];

export function mount(root) {
  const el = h('div', { class: 'page' });
  root.append(el);

  const card = (title, sub, ...content) => h('div', { class: 'card' },
    h('div', { class: 'card-head' }, h('div', null, h('h2', null, title), sub ? h('p', { class: 'card-sub' }, sub) : null)), ...content);
  const defs = (items) => h('dl', { class: 'about-list' },
    items.map(([term, text]) => [h('dt', null, term), h('dd', null, text)]).flat());
  const link = (href, ic, text) => h('a', { class: 'btn btn-sm', href, target: '_blank', rel: 'noopener noreferrer', title: text }, icon(ic), text);

  function render() {
    const version = [store.version, store.commit].filter(Boolean).join(' · ') || 'development build';
    el.replaceChildren(
      h('div', { class: 'page-head' }, h('div', null, h('h1', null, `About ${APP}`), h('p', null, TAGLINE))),
      h('div', { class: 'card about-hero' },
        h('img', { src: '/appicon.png', width: 64, height: 64, alt: '' }),
        h('div', null,
          h('h2', null, APP),
          h('p', null, DESCRIPTION),
          h('div', { class: 'about-badges' },
            h('span', { class: 'badge' }, icon('server'), version),
            h('span', { class: 'badge' }, icon('lock'), LICENSE)))),

      h('div', { class: 'grid grid-2e mt' },
        card('Why it exists', 'The problem this was written for', defs(PROBLEMS)),
        card('What it does', 'The short version', defs(FEATURES))),

      h('div', { class: 'grid grid-2 mt' },
        card('Technology', 'What it is built on and why', defs(STACK)),
        card('Author', 'Built and maintained by',
          h('div', { class: 'about-author' },
            h('div', { class: 'about-avatar' }, AUTHOR.name.split(' ').map((w) => w[0]).slice(0, 2).join('')),
            h('div', null,
              h('div', { class: 'strong' }, AUTHOR.name),
              h('div', { class: 'muted' }, `github.com/${AUTHOR.github}`),
              h('div', { class: 'muted' }, AUTHOR.email))),
          h('div', { class: 'about-links' },
            link(`https://github.com/${AUTHOR.github}`, 'github', 'GitHub profile'),
            link(`mailto:${AUTHOR.email}`, 'mail', 'Send an email')))),

      h('div', { class: 'grid grid-2e mt' },
        card('Built with AI assistance', 'Written by a human, with models in the loop',
          h('p', { class: 'about-text' },
            'Parts of this app were designed and written with the help of AI models. ',
            'Every suggestion was reviewed, tested and adjusted by hand before it shipped.')),
        card('Credits', 'Open source this app is built on',
          h('p', { class: 'note' }, icon('sparkles'),
            'Fonts by JetBrains (SIL OFL) and icons from Lucide (ISC). Thanks to both projects.'))));
  }

  render();
  return { refresh: render, destroy() {} };
}
```

## Lampiran D: auth-page.js

Setup awal, login bertahap, dan recovery. Membutuhkan endpoint [§12.7](#127-endpoint).

### `web/static/js/auth-page.js`

```js
// /setup (first run), /login and account recovery. Sign-in is a chain of
// steps decided by the server (code, new password, authenticator setup); the
// server sets an HttpOnly session cookie once the chain is done.
import { h, icon, copy, passwordProblem, passwordRules } from './ui.js';

const APP = 'App';

const wrap = document.getElementById('card-wrap');
const card = document.getElementById('card');
const params = new URLSearchParams(location.search);

document.getElementById('copyright').textContent = `© ${new Date().getFullYear()} ${APP}`;
document.getElementById('theme').addEventListener('click', () => {
  const t = document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark';
  document.documentElement.dataset.theme = t;
  try { localStorage.setItem('app_theme', t); } catch { /* private mode */ }
});

// Only local paths; the #fragment survives the server redirect and is kept.
function nextURL() {
  const n = params.get('next') || '/';
  const path = n.startsWith('/') && !n.startsWith('//') ? n : '/';
  return path + location.hash;
}

async function post(path, body) {
  let resp;
  try {
    resp = await fetch('/api/v1' + path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    });
  } catch {
    throw new Error(`Cannot reach the ${APP} server`);
  }
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) throw Object.assign(new Error(data.error || `Request failed (HTTP ${resp.status})`), { expired: !!data.expired });
  return data;
}

function field(label, attrs) {
  const input = h('input', { class: 'input', required: true, spellcheck: 'false', ...attrs });
  return { input, el: h('label', { class: 'field' }, h('span', null, label), input) };
}

const title = (text, sub) => h('div', { class: 'auth-title' }, h('h1', null, text), h('p', null, sub));
const submit = (text) => h('button', { class: 'btn btn-primary btn-block', type: 'submit' }, text);
const note = (kind, text) => h('p', { class: `auth-note ${kind}` }, icon(kind === 'warn' ? 'alert' : 'help'), h('span', null, text));
const hint30 = () => h('p', { class: 'auth-hint' }, icon('lock'), ' You stay signed in on this browser for 30 days.');

async function busy(btn, text, fn) {
  const old = btn.textContent;
  btn.disabled = true;
  btn.textContent = text;
  try { await fn(); } finally { btn.disabled = false; btn.textContent = old; }
}

// Six single-digit boxes; typing, pasting and one-time-code autofill all work.
function otpInput(onComplete) {
  const boxes = Array.from({ length: 6 }, (_, i) => h('input', {
    class: 'otp-box', inputmode: 'numeric', autocomplete: i === 0 ? 'one-time-code' : 'off', 'aria-label': `Digit ${i + 1}`,
  }));
  const value = () => boxes.map((b) => b.value).join('');
  const fill = (digits, from) => {
    digits.split('').forEach((c, j) => { if (boxes[from + j]) boxes[from + j].value = c; });
    boxes[Math.min(from + digits.length, 5)].focus();
    if (value().length === 6) onComplete(value());
  };
  boxes.forEach((b, i) => {
    b.addEventListener('input', () => {
      const digits = b.value.replace(/\D/g, '');
      b.value = '';
      if (digits) fill(digits.slice(0, 6 - i), i);
    });
    b.addEventListener('keydown', (e) => {
      if (e.key === 'Backspace' && !b.value && i > 0) boxes[i - 1].focus();
      if (e.key === 'ArrowLeft' && i > 0) boxes[i - 1].focus();
      if (e.key === 'ArrowRight' && i < 5) boxes[i + 1].focus();
    });
    b.addEventListener('paste', (e) => {
      const digits = (e.clipboardData.getData('text') || '').replace(/\D/g, '').slice(0, 6);
      if (!digits) return;
      e.preventDefault();
      boxes.forEach((x) => { x.value = ''; });
      fill(digits, 0);
    });
  });
  return {
    el: h('div', { class: 'otp-row' }, boxes),
    value,
    clear() { boxes.forEach((b) => { b.value = ''; }); boxes[0].focus(); },
    focus() { boxes[0].focus(); },
  };
}

const otpField = (label, otp) => h('div', { class: 'field center' }, h('span', null, label), otp.el);

function qrBlock(qrURL, secret) {
  return h('div', { class: 'qr' },
    qrURL ? h('img', { src: qrURL, alt: 'QR code for your authenticator app' }) : h('div', { class: 'skel', style: { width: '140px', height: '140px' } }),
    h('div', null,
      h('div', { class: 'muted' }, 'Cannot scan? Enter this key:'),
      h('code', null, secret || '...'),
      secret ? h('button', { class: 'btn btn-sm', type: 'button', onclick: () => copy(secret) }, icon('copy'), 'Copy key') : null));
}

function questionSelect(questions) {
  return h('select', { class: 'select wide input', required: true },
    h('option', { value: '' }, 'Choose a question'), questions.map((q) => h('option', { value: q.id }, q.text)));
}

function step(iconName, heading, sub) {
  const ic = h('div', { class: 'step-icon' }, icon(iconName));
  const body = h('div', { class: 'step-body' });
  const el = h('section', { class: 'step' }, ic, h('div', { class: 'step-head' }, h('h2', null, heading), h('p', null, sub)), body);
  return {
    el, body,
    state(s) {
      el.className = `step ${s}`;
      ic.replaceChildren(icon(s === 'done' ? 'checkmark' : iconName));
      body.hidden = s !== 'active';
    },
  };
}

// ── First-run setup: account, recovery question, authenticator ──
async function renderSetup() {
  wrap.classList.add('wide');
  let questions = [];
  try { questions = await (await fetch('/api/v1/auth/questions')).json(); } catch { /* the select stays empty */ }

  const user = field('Username', { value: 'admin', autocomplete: 'username' });
  const pass = field('Password', { type: 'password', minlength: 8, autocomplete: 'new-password', placeholder: 'e.g. Qawsed#1477' });
  const pass2 = field('Confirm password', { type: 'password', minlength: 8, autocomplete: 'new-password' });
  const err1 = h('p', { class: 'form-error' });
  const next1 = submit('Continue');
  const s1 = step('user', 'Administrator account', 'Administrators manage users; you can add operators later');
  const form1 = h('form', { class: 'auth-form' }, user.el, pass.el, passwordRules(pass.input), pass2.el, next1, err1);
  s1.body.append(form1);

  const question = questionSelect(questions);
  const answer = field('Answer', { autocomplete: 'off', placeholder: 'Not case sensitive' });
  const err2 = h('p', { class: 'form-error' });
  const next2 = submit('Continue');
  const back2 = h('button', { class: 'link-btn', type: 'button' }, 'Back');
  const s2 = step('help', 'Recovery question', 'Lets you reset a lost password or phone from the sign-in page');
  const form2 = h('form', { class: 'auth-form' }, h('label', { class: 'field' }, h('span', null, 'Question'), question), answer.el, next2, err2,
    h('div', { class: 'auth-links' }, back2));
  s2.body.append(form2);

  const s3 = step('shield', 'Two-factor sign-in', 'Scan the QR code with Google Authenticator, Authy or 1Password');
  const err3 = h('p', { class: 'form-error' });
  const finish = submit('Enable 2FA and sign in');
  const back3 = h('button', { class: 'link-btn', type: 'button' }, 'Back');
  let secret = '';
  const complete = () => busy(finish, 'Verifying...', async () => {
    err3.textContent = '';
    try {
      await post('/auth/setup/complete', {
        username: user.input.value.trim(), password: pass.input.value, secret, code: otp.value(),
        question: question.value, answer: answer.input.value,
      });
      location.replace(nextURL());
    } catch (e) {
      err3.textContent = e.message;
      otp.clear();
    }
  });
  const otp = otpInput(complete);
  const qrBox = h('div', null, qrBlock('', ''));
  const form3 = h('form', { class: 'auth-form' }, qrBox, otpField('Enter the 6-digit code shown in the app', otp), finish, err3,
    h('div', { class: 'auth-links' }, back3));
  s3.body.append(form3);

  card.replaceChildren(title('Initial setup', 'Create the administrator account and turn on two-factor sign-in'),
    h('div', { class: 'steps' }, s1.el, s2.el, s3.el));
  const go = (n) => {
    [s1, s2, s3].forEach((s, i) => s.state(i + 1 < n ? 'done' : i + 1 === n ? 'active' : 'pending'));
    ({ 1: () => pass.input.focus(), 2: () => question.focus(), 3: () => otp.focus() })[n]();
  };
  go(1);
  user.input.focus();

  form1.addEventListener('submit', (e) => {
    e.preventDefault();
    err1.textContent = '';
    const problem = passwordProblem(pass.input.value);
    if (problem) return void (err1.textContent = problem);
    if (pass.input.value !== pass2.input.value) return void (err1.textContent = 'Passwords do not match.');
    go(2);
  });
  form2.addEventListener('submit', (e) => {
    e.preventDefault();
    err2.textContent = '';
    if (!question.value) return void (err2.textContent = 'Choose a question.');
    if (answer.input.value.trim().length < 3) return void (err2.textContent = 'The answer must be at least 3 characters.');
    busy(next2, 'Preparing...', async () => {
      try {
        if (!secret) {
          const init = await post('/auth/setup/init');
          secret = init.secret;
          qrBox.replaceChildren(qrBlock(init.qr_data_url, secret));
        }
        go(3);
      } catch (e2) {
        err2.textContent = e2.message;
      }
    });
  });
  form3.addEventListener('submit', (e) => { e.preventDefault(); complete(); });
  back2.addEventListener('click', () => go(1));
  back3.addEventListener('click', () => go(2));
}

// ── Sign in and recovery ──
function renderLogin() {
  let temp = '';
  let username = '';
  let timer = null;

  const stopTimer = () => clearTimeout(timer);
  // The server forgets a half-done sign-in after a few minutes: go back to the
  // password form and say why there, where the user has to act.
  function startTimer(seconds) {
    stopTimer();
    if (seconds > 0) timer = setTimeout(() => showCredentials('Your sign-in took too long. Enter your password again.'), seconds * 1000);
  }

  function show(wide, ...nodes) {
    wrap.classList.toggle('wide', wide);
    card.replaceChildren(...nodes);
  }

  function fail(e, err, otp) {
    if (e.expired) return showCredentials(e.message);
    err.textContent = e.message;
    otp?.clear();
  }

  function onStep(res) {
    if (res.step === 'done') {
      stopTimer();
      location.replace(nextURL());
      return;
    }
    temp = res.temp_token;
    username = res.username || username;
    startTimer(res.expires_in);
    const screens = { totp: showTOTP, password: showNewPassword, enroll: showEnroll };
    (screens[res.step] || (() => showCredentials('Unexpected sign-in step, start again.')))(res);
  }

  const accountChip = () => h('div', { class: 'account-chip' },
    h('span', null, icon('user'), ' ', h('b', null, username)),
    h('button', { class: 'link-btn', type: 'button', onclick: () => showCredentials() }, 'Change'));

  function showCredentials(notice = '') {
    stopTimer();
    temp = '';
    const user = field('Username', { autocomplete: 'username', value: username });
    const pass = field('Password', { type: 'password', autocomplete: 'current-password' });
    const err = h('p', { class: 'form-error' });
    const btn = submit('Continue');
    const forgot = h('button', { class: 'link-btn', type: 'button', onclick: () => showRecoverStart(user.input.value.trim()) }, 'Forgot your password or lost your phone?');
    const form = h('form', { class: 'auth-form' }, notice ? note('warn', notice) : null, user.el, pass.el, btn, err, h('div', { class: 'auth-links' }, forgot));
    show(false, title('Sign in', `Welcome back to ${APP}`), form, hint30());
    (user.input.value ? pass.input : user.input).focus();
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      err.textContent = '';
      busy(btn, 'Checking...', async () => {
        try {
          username = user.input.value.trim();
          onStep(await post('/auth/login/credentials', { username, password: pass.input.value }));
        } catch (e2) {
          err.textContent = e2.message;
          pass.input.select();
        }
      });
    });
  }

  function showTOTP() {
    const err = h('p', { class: 'form-error' });
    const btn = submit('Verify and sign in');
    const go = () => busy(btn, 'Verifying...', async () => {
      err.textContent = '';
      try { onStep(await post('/auth/login/2fa', { temp_token: temp, code: otp.value() })); } catch (e) { fail(e, err, otp); }
    });
    const otp = otpInput(go);
    const form = h('form', { class: 'auth-form' }, accountChip(), otpField('Code from your authenticator app', otp), btn, err);
    form.addEventListener('submit', (e) => { e.preventDefault(); go(); });
    show(false, title('Two-factor authentication', 'Enter the 6-digit code to finish signing in'), form);
    otp.focus();
  }

  function showNewPassword() {
    const pass = field('New password', { type: 'password', minlength: 8, autocomplete: 'new-password', placeholder: 'e.g. Qawsed#1477' });
    const pass2 = field('Confirm new password', { type: 'password', minlength: 8, autocomplete: 'new-password' });
    const err = h('p', { class: 'form-error' });
    const btn = submit('Save password and continue');
    const form = h('form', { class: 'auth-form' }, accountChip(), pass.el, passwordRules(pass.input), pass2.el, btn, err);
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      err.textContent = '';
      const problem = passwordProblem(pass.input.value);
      if (problem) return void (err.textContent = problem);
      if (pass.input.value !== pass2.input.value) return void (err.textContent = 'Passwords do not match.');
      busy(btn, 'Saving...', async () => {
        try { onStep(await post('/auth/login/password', { temp_token: temp, password: pass.input.value })); } catch (e2) { fail(e2, err); }
      });
    });
    show(false, title('Choose a new password', 'Your current password is temporary'), form);
    pass.input.focus();
  }

  function showEnroll(res) {
    const err = h('p', { class: 'form-error' });
    const btn = submit('Enable 2FA and continue');
    const go = () => busy(btn, 'Verifying...', async () => {
      err.textContent = '';
      try { onStep(await post('/auth/login/enroll', { temp_token: temp, code: otp.value() })); } catch (e) { fail(e, err, otp); }
    });
    const otp = otpInput(go);
    const form = h('form', { class: 'auth-form' }, accountChip(), qrBlock(res.qr_data_url, res.secret),
      otpField('Enter the 6-digit code shown in the app', otp), btn, err);
    form.addEventListener('submit', (e) => { e.preventDefault(); go(); });
    show(true, title('Set up two-factor sign-in', 'Scan the QR code with Google Authenticator, Authy or 1Password'), form);
    otp.focus();
  }

  function showRecoverStart(prefill) {
    stopTimer();
    const user = field('Username', { autocomplete: 'username', value: prefill || username });
    const err = h('p', { class: 'form-error' });
    const btn = submit('Continue');
    const back = h('button', { class: 'link-btn', type: 'button', onclick: () => showCredentials() }, 'Back to sign in');
    const form = h('form', { class: 'auth-form' },
      note('info', 'Administrators answer their recovery question. Operators: ask an administrator to reset your account.'),
      user.el, btn, err, h('div', { class: 'auth-links' }, back));
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      err.textContent = '';
      busy(btn, 'Checking...', async () => {
        try {
          const res = await post('/auth/recover/start', { username: user.input.value.trim() });
          temp = res.temp_token;
          username = res.username;
          startTimer(res.expires_in);
          showRecoverAnswer(res.question);
        } catch (e2) {
          err.textContent = e2.message;
        }
      });
    });
    show(false, title('Recover your account', 'Reset your password or your two-factor sign-in'), form);
    user.input.focus();
  }

  function showRecoverAnswer(question) {
    let mode = 'password';
    const answer = field(question, { autocomplete: 'off', placeholder: 'Your answer (not case sensitive)' });
    const err = h('p', { class: 'form-error' });
    const btn = submit('Verify');
    const go = () => busy(btn, 'Verifying...', async () => {
      err.textContent = '';
      try {
        onStep(await post('/auth/recover/verify', {
          temp_token: temp, answer: answer.input.value, mode,
          code: mode === 'password' ? otp.value() : '', password: mode === '2fa' ? pass.input.value : '',
        }));
      } catch (e) {
        fail(e, err, mode === 'password' ? otp : null);
      }
    });
    const otp = otpInput(() => { if (answer.input.value.trim()) go(); });
    const pass = field('Your password', { type: 'password', autocomplete: 'current-password' });
    const factor = h('div');
    const choice = (value, label, sub) => {
      const radio = h('input', { type: 'radio', name: 'mode', value, checked: value === mode });
      const el = h('label', { class: `choice-item${value === mode ? ' on' : ''}` }, radio, h('span', null, label, h('small', null, sub)));
      radio.addEventListener('change', () => { mode = value; sync(); });
      return el;
    };
    const choices = h('div', { class: 'choice' },
      choice('password', 'I forgot my password', 'Confirm with your 2FA code'),
      choice('2fa', 'I lost my phone', 'Confirm with your password'));
    function sync() {
      choices.querySelectorAll('.choice-item').forEach((c) => c.classList.toggle('on', c.querySelector('input').value === mode));
      factor.replaceChildren(mode === 'password' ? otpField('Current code from your authenticator app', otp) : pass.el);
      pass.input.required = mode === '2fa';
    }
    sync();
    const form = h('form', { class: 'auth-form' }, accountChip(), answer.el, choices, factor, btn, err,
      h('div', { class: 'auth-links' }, h('button', { class: 'link-btn', type: 'button', onclick: () => showCredentials() }, 'Back to sign in')));
    form.addEventListener('submit', (e) => { e.preventDefault(); go(); });
    show(false, title('Answer your recovery question', 'Plus one thing only you still have'), form);
    answer.input.focus();
  }

  showCredentials();
}

async function start() {
  let st;
  try {
    st = await (await fetch('/api/v1/auth/status')).json();
  } catch {
    card.replaceChildren(title(`${APP} is unreachable`, 'Check that the server is running, then reload this page.'));
    return;
  }
  if (!st.auth_enabled || st.authenticated) return location.replace(nextURL());
  if (st.setup_needed) {
    if (location.pathname !== '/setup') history.replaceState(null, '', '/setup' + location.hash);
    document.title = `Setup - ${APP}`;
    renderSetup();
  } else {
    renderLogin();
  }
}

start();
```
