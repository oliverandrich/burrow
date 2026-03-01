# Third-Party Licenses

This file lists all third-party components used by Burrow, including embedded
assets and Go module dependencies.

To regenerate the Go dependency section, run:

```bash
go-licenses report ./... 2>/dev/null | sort
```

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

- **Files:** `contrib/bootstrap/static/htmx.min.js`
- **URL:** <https://htmx.org/>
- **Copyright:** Copyright (c) 2020, Big Sky Software LLC
- **License:** BSD 2-Clause — <https://github.com/bigskysoftware/htmx/blob/master/LICENSE>

## Go Module Dependencies

Generated with [google/go-licenses](https://github.com/google/go-licenses).

| Module | License | URL |
|--------|---------|-----|
| github.com/BurntSushi/toml v1.6.0 | MIT | [COPYING](https://github.com/BurntSushi/toml/blob/v1.6.0/COPYING) |
| github.com/a-h/templ v0.3.1001 | MIT | [LICENSE](https://github.com/a-h/templ/blob/v0.3.1001/LICENSE) |
| github.com/dustin/go-humanize v1.0.1 | MIT | [LICENSE](https://github.com/dustin/go-humanize/blob/v1.0.1/LICENSE) |
| github.com/fxamacker/cbor/v2 v2.9.0 | MIT | [LICENSE](https://github.com/fxamacker/cbor/blob/v2.9.0/LICENSE) |
| github.com/go-chi/chi/v5 v5.2.5 | MIT | [LICENSE](https://github.com/go-chi/chi/blob/v5.2.5/LICENSE) |
| github.com/go-chi/httplog/v3 v3.3.0 | MIT | [LICENSE](https://github.com/go-chi/httplog/blob/v3.3.0/LICENSE) |
| github.com/go-viper/mapstructure/v2 v2.5.0 | MIT | [LICENSE](https://github.com/go-viper/mapstructure/blob/v2.5.0/LICENSE) |
| github.com/go-webauthn/webauthn v0.15.0 | BSD-3-Clause | [LICENSE](https://github.com/go-webauthn/webauthn/blob/v0.15.0/LICENSE) |
| github.com/go-webauthn/x v0.2.0 | BSD-2-Clause | [LICENSE](https://github.com/go-webauthn/x/blob/v0.2.0/revoke/LICENSE) |
| github.com/golang-jwt/jwt/v5 v5.3.1 | MIT | [LICENSE](https://github.com/golang-jwt/jwt/blob/v5.3.1/LICENSE) |
| github.com/google/go-tpm v0.9.8 | Apache-2.0 | [LICENSE](https://github.com/google/go-tpm/blob/v0.9.8/LICENSE) |
| github.com/google/uuid v1.6.0 | BSD-3-Clause | [LICENSE](https://github.com/google/uuid/blob/v1.6.0/LICENSE) |
| github.com/gorilla/csrf v1.7.3 | BSD-3-Clause | [LICENSE](https://github.com/gorilla/csrf/blob/v1.7.3/LICENSE) |
| github.com/gorilla/securecookie v1.1.2 | BSD-3-Clause | [LICENSE](https://github.com/gorilla/securecookie/blob/v1.1.2/LICENSE) |
| github.com/jinzhu/inflection v1.0.0 | MIT | [LICENSE](https://github.com/jinzhu/inflection/blob/v1.0.0/LICENSE) |
| github.com/mattn/go-isatty v0.0.20 | MIT | [LICENSE](https://github.com/mattn/go-isatty/blob/v0.0.20/LICENSE) |
| github.com/ncruces/go-strftime v1.0.0 | MIT | [LICENSE](https://github.com/ncruces/go-strftime/blob/v1.0.0/LICENSE) |
| github.com/nicksnyder/go-i18n/v2 v2.6.1 | MIT | [LICENSE](https://github.com/nicksnyder/go-i18n/blob/v2.6.1/LICENSE) |
| github.com/puzpuzpuz/xsync/v3 v3.5.1 | Apache-2.0 | [LICENSE](https://github.com/puzpuzpuz/xsync/blob/v3.5.1/LICENSE) |
| github.com/remyoudompheng/bigfft v0.0.0 | BSD-3-Clause | [LICENSE](https://github.com/remyoudompheng/bigfft/blob/24d4a6f8daec/LICENSE) |
| github.com/stretchr/testify v1.11.1 | MIT | [LICENSE](https://github.com/stretchr/testify/blob/v1.11.1/LICENSE) |
| github.com/tmthrgd/go-hex v0.0.0 | BSD-2-Clause | [LICENSE](https://github.com/tmthrgd/go-hex/blob/447a3041c3bc/LICENSE) |
| github.com/uptrace/bun v1.2.16 | BSD-2-Clause | [LICENSE](https://github.com/uptrace/bun/blob/v1.2.16/LICENSE) |
| github.com/urfave/cli/v3 v3.6.2 | MIT | [LICENSE](https://github.com/urfave/cli/blob/v3.6.2/LICENSE) |
| github.com/vmihailenco/msgpack/v5 v5.4.1 | BSD-2-Clause | [LICENSE](https://github.com/vmihailenco/msgpack/blob/v5.4.1/LICENSE) |
| github.com/vmihailenco/tagparser/v2 v2.0.0 | BSD-2-Clause | [LICENSE](https://github.com/vmihailenco/tagparser/blob/v2.0.0/LICENSE) |
| github.com/x448/float16 v0.8.4 | MIT | [LICENSE](https://github.com/x448/float16/blob/v0.8.4/LICENSE) |
| golang.org/x/crypto v0.48.0 | BSD-3-Clause | [LICENSE](https://cs.opensource.google/go/x/crypto/+/v0.48.0:LICENSE) |
| golang.org/x/exp v0.0.0 | BSD-3-Clause | [LICENSE](https://cs.opensource.google/go/x/exp/+/81e46e3d:LICENSE) |
| golang.org/x/sys v0.41.0 | BSD-3-Clause | [LICENSE](https://cs.opensource.google/go/x/sys/+/v0.41.0:LICENSE) |
| golang.org/x/text v0.34.0 | BSD-3-Clause | [LICENSE](https://cs.opensource.google/go/x/text/+/v0.34.0:LICENSE) |
| modernc.org/libc v1.67.7 | MIT | [LICENSE](https://gitlab.com/cznic/libc/blob/v1.67.7/LICENSE-3RD-PARTY.md) |
| modernc.org/memory v1.11.0 | BSD-3-Clause | [LICENSE](https://gitlab.com/cznic/memory/blob/v1.11.0/LICENSE-GO) |
| modernc.org/sqlite v1.45.0 | BSD-3-Clause | [LICENSE](https://gitlab.com/cznic/sqlite/blob/v1.45.0/LICENSE) |
