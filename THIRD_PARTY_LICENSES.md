# Third-Party Licenses

This file lists all third-party components used by Burrow, including embedded
assets and Go module dependencies.

This file is auto-generated. Run `just licenses` to regenerate it.

## Embedded Assets

These files are bundled directly into the binary and do not carry their license
headers in every distributed form.

### Bootstrap v5.3.8

- **Files:** `contrib/bootstrap/static/bootstrap.min.css`, `contrib/bootstrap/static/bootstrap.bundle.min.js`
- **URL:** <https://getbootstrap.com/>
- **Copyright:** Copyright 2011-2025 The Bootstrap Authors
- **License:** MIT — <https://github.com/twbs/bootstrap/blob/main/LICENSE>

### Bootstrap Icons v1.13.1

- **Files:** `contrib/bsicons/icons_generated.go` (inline SVG paths)
- **URL:** <https://icons.getbootstrap.com/>
- **Copyright:** Copyright 2019-2025 The Bootstrap Authors
- **License:** MIT — <https://github.com/twbs/icons/blob/main/LICENSE>

### htmx v2.0.8

- **Files:** `contrib/htmx/static/htmx.min.js`
- **URL:** <https://htmx.org/>
- **Copyright:** Copyright (c) 2020, Big Sky Software LLC
- **License:** BSD 2-Clause — <https://github.com/bigskysoftware/htmx/blob/master/LICENSE>

## Development Tools

### just

- **URL:** <https://just.systems/>
- **Copyright:** Copyright (c) Casey Rodarmor
- **License:** CC0-1.0 — <https://github.com/casey/just/blob/master/LICENSE>

### zensical

- **URL:** <https://zensical.org/>
- **Copyright:** Copyright (c) Zensical contributors
- **License:** MIT — <https://pypi.org/project/zensical/>

## Go Module Dependencies

Generated with [google/go-licenses](https://github.com/google/go-licenses).

| Module | License | URL |
|--------|---------|-----|
| github.com/BurntSushi/toml | MIT | [LICENSE](https://github.com/BurntSushi/toml/blob/v1.6.0/COPYING) |
| github.com/cloudflare/tableflip | BSD-3-Clause | [LICENSE](https://github.com/cloudflare/tableflip/blob/v1.2.3/LICENSE) |
| github.com/davecgh/go-spew/spew | ISC | [LICENSE](https://github.com/davecgh/go-spew/blob/v1.1.1/LICENSE) |
| github.com/dustin/go-humanize | MIT | [LICENSE](https://github.com/dustin/go-humanize/blob/v1.0.1/LICENSE) |
| github.com/fxamacker/cbor/v2 | MIT | [LICENSE](https://github.com/fxamacker/cbor/blob/v2.9.0/LICENSE) |
| github.com/gabriel-vasile/mimetype | MIT | [LICENSE](https://github.com/gabriel-vasile/mimetype/blob/v1.4.13/LICENSE) |
| github.com/go-chi/chi/v5 | MIT | [LICENSE](https://github.com/go-chi/chi/blob/v5.2.5/LICENSE) |
| github.com/go-chi/httplog/v3 | MIT | [LICENSE](https://github.com/go-chi/httplog/blob/v3.3.0/LICENSE) |
| github.com/go-playground/form/v4 | MIT | [LICENSE](https://github.com/go-playground/form/blob/v4.3.0/LICENSE) |
| github.com/go-playground/locales | MIT | [LICENSE](https://github.com/go-playground/locales/blob/v0.14.1/LICENSE) |
| github.com/go-playground/universal-translator | MIT | [LICENSE](https://github.com/go-playground/universal-translator/blob/v0.18.1/LICENSE) |
| github.com/go-playground/validator/v10 | MIT | [LICENSE](https://github.com/go-playground/validator/blob/v10.30.1/LICENSE) |
| github.com/go-viper/mapstructure/v2 | MIT | [LICENSE](https://github.com/go-viper/mapstructure/blob/v2.5.0/LICENSE) |
| github.com/go-webauthn/webauthn | BSD-3-Clause | [LICENSE](https://github.com/go-webauthn/webauthn/blob/v0.16.1/LICENSE) |
| github.com/go-webauthn/x/encoding/asn1 | BSD-3-Clause | [LICENSE](https://github.com/go-webauthn/x/blob/v0.2.2/LICENSE) |
| github.com/go-webauthn/x/revoke | BSD-2-Clause | [LICENSE](https://github.com/go-webauthn/x/blob/v0.2.2/revoke/LICENSE) |
| github.com/golang-jwt/jwt/v5 | MIT | [LICENSE](https://github.com/golang-jwt/jwt/blob/v5.3.1/LICENSE) |
| github.com/google/go-tpm | Apache-2.0 | [LICENSE](https://github.com/google/go-tpm/blob/v0.9.8/LICENSE) |
| github.com/google/uuid | BSD-3-Clause | [LICENSE](https://github.com/google/uuid/blob/v1.6.0/LICENSE) |
| github.com/gorilla/csrf | BSD-3-Clause | [LICENSE](https://github.com/gorilla/csrf/blob/v1.7.3/LICENSE) |
| github.com/gorilla/securecookie | BSD-3-Clause | [LICENSE](https://github.com/gorilla/securecookie/blob/v1.1.2/LICENSE) |
| github.com/jinzhu/inflection | MIT | [LICENSE](https://github.com/jinzhu/inflection/blob/v1.0.0/LICENSE) |
| github.com/leodido/go-urn | MIT | [LICENSE](https://github.com/leodido/go-urn/blob/v1.4.0/LICENSE) |
| github.com/mattn/go-isatty | MIT | [LICENSE](https://github.com/mattn/go-isatty/blob/v0.0.20/LICENSE) |
| github.com/ncruces/go-strftime | MIT | [LICENSE](https://github.com/ncruces/go-strftime/blob/v1.0.0/LICENSE) |
| github.com/nicksnyder/go-i18n/v2 | MIT | [LICENSE](https://github.com/nicksnyder/go-i18n/blob/v2.6.1/LICENSE) |
| github.com/pmezard/go-difflib/difflib | BSD-3-Clause | [LICENSE](https://github.com/pmezard/go-difflib/blob/v1.0.0/LICENSE) |
| github.com/puzpuzpuz/xsync/v3 | Apache-2.0 | [LICENSE](https://github.com/puzpuzpuz/xsync/blob/v3.5.1/LICENSE) |
| github.com/remyoudompheng/bigfft | BSD-3-Clause | [LICENSE](https://github.com/remyoudompheng/bigfft/blob/24d4a6f8daec/LICENSE) |
| github.com/stretchr/testify | MIT | [LICENSE](https://github.com/stretchr/testify/blob/v1.11.1/LICENSE) |
| github.com/tmthrgd/go-hex | BSD-2-Clause | [LICENSE](https://github.com/tmthrgd/go-hex/blob/447a3041c3bc/LICENSE) |
| github.com/unrolled/secure | MIT | [LICENSE](https://github.com/unrolled/secure/blob/v1.17.0/LICENSE) |
| github.com/uptrace/bun | BSD-2-Clause | [LICENSE](https://github.com/uptrace/bun/blob/v1.2.18/LICENSE) |
| github.com/uptrace/bun/dialect/sqlitedialect | BSD-2-Clause | [LICENSE](https://github.com/uptrace/bun/blob/dialect/sqlitedialect/v1.2.18/dialect/sqlitedialect/LICENSE) |
| github.com/uptrace/bun/driver/sqliteshim | BSD-2-Clause | [LICENSE](https://github.com/uptrace/bun/blob/driver/sqliteshim/v1.2.18/driver/sqliteshim/LICENSE) |
| github.com/urfave/cli/v3 | MIT | [LICENSE](https://github.com/urfave/cli/blob/v3.7.0/LICENSE) |
| github.com/vmihailenco/msgpack/v5 | BSD-2-Clause | [LICENSE](https://github.com/vmihailenco/msgpack/blob/v5.4.1/LICENSE) |
| github.com/vmihailenco/tagparser/v2 | BSD-2-Clause | [LICENSE](https://github.com/vmihailenco/tagparser/blob/v2.0.0/LICENSE) |
| github.com/wneessen/go-mail | MIT | [LICENSE](https://github.com/wneessen/go-mail/blob/v0.7.2/LICENSE) |
| github.com/wneessen/go-mail/smtp | BSD-3-Clause | [LICENSE](https://github.com/wneessen/go-mail/blob/v0.7.2/smtp/LICENSE) |
| github.com/x448/float16 | MIT | [LICENSE](https://github.com/x448/float16/blob/v0.8.4/LICENSE) |
| golang.org/x/crypto | BSD-3-Clause | [LICENSE](https://cs.opensource.google/go/x/crypto/+/v0.49.0:LICENSE) |
| golang.org/x/net/idna | BSD-3-Clause | [LICENSE](https://cs.opensource.google/go/x/net/+/v0.52.0:LICENSE) |
| golang.org/x/sys | BSD-3-Clause | [LICENSE](https://cs.opensource.google/go/x/sys/+/v0.42.0:LICENSE) |
| golang.org/x/text | BSD-3-Clause | [LICENSE](https://cs.opensource.google/go/x/text/+/v0.35.0:LICENSE) |
| golang.org/x/time/rate | BSD-3-Clause | [LICENSE](https://cs.opensource.google/go/x/time/+/v0.15.0:LICENSE) |
| gopkg.in/yaml.v3 | MIT | [LICENSE](https://github.com/go-yaml/yaml/blob/v3.0.1/LICENSE) |
| modernc.org/libc | MIT | [LICENSE](https://gitlab.com/cznic/libc/blob/v1.70.0/LICENSE-3RD-PARTY.md) |
| modernc.org/mathutil | BSD-3-Clause | [LICENSE](https://gitlab.com/cznic/mathutil/-/blob/master/LICENSE) |
| modernc.org/memory | BSD-3-Clause | [LICENSE](https://gitlab.com/cznic/memory/blob/v1.11.0/LICENSE-GO) |
| modernc.org/sqlite | BSD-3-Clause | [LICENSE](https://gitlab.com/cznic/sqlite/blob/v1.46.1/LICENSE) |
