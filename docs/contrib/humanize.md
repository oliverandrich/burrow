# Humanize

Human-friendly display of times, numbers, and file sizes in templates.

**Package:** `github.com/oliverandrich/burrow/contrib/humanize`

Inspired by Django's [django.contrib.humanize](https://docs.djangoproject.com/en/6.0/ref/contrib/humanize/).

## Setup

Register the humanize app in your server:

```go
srv := burrow.NewServer(
    humanize.New(),
    // ... other apps
)
```

All template functions become available in your templates automatically.

### Options

```go
humanize.New(
    humanize.WithDateFormat("2006-01-02"), // custom fallback date format for naturalday
)
```

## Template Functions

### `apnumber`

For numbers 1–9, returns the number spelled out. Otherwise, returns the number as digits. This follows Associated Press style.

Examples:

- `{{ apnumber 1 }}` → `one`
- `{{ apnumber 5 }}` → `five`
- `{{ apnumber 9 }}` → `nine`
- `{{ apnumber 10 }}` → `10`
- `{{ apnumber 0 }}` → `0`

With German locale:

- `{{ apnumber 1 }}` → `eins`
- `{{ apnumber 5 }}` → `fünf`

### `intcomma`

Formats an integer with locale-aware thousands separators using `golang.org/x/text/message`.

Examples:

- `{{ intcomma 4500 }}` → `4,500`
- `{{ intcomma 45000 }}` → `45,000`
- `{{ intcomma 450000 }}` → `450,000`
- `{{ intcomma 4500000 }}` → `4,500,000`

With German locale:

- `{{ intcomma 45000 }}` → `45.000`
- `{{ intcomma 4500000 }}` → `4.500.000`

### `naturalday`

For dates that are today, yesterday, or tomorrow, returns a translated word. Otherwise, formats the date using a locale-specific format.

```html
{{ naturalday .CreatedAt }}
{{ naturalday .PublishedAt }}
```

Examples (when today is April 1, 2026):

- Today → `today`
- Yesterday → `yesterday`
- Tomorrow → `tomorrow`
- Any other date → `Mar 15, 2026`

With German locale:

- Today → `heute`
- Yesterday → `gestern`
- Tomorrow → `morgen`
- Any other date → `15.03.2026`

The fallback date format can be customized with `WithDateFormat`. Default formats:

- English: `Jan 2, 2006`
- German: `02.01.2006`

### `naturaltime`

For a `time.Time` value, returns a string representing how many seconds, minutes, hours, days, weeks, months, or years ago it was. For future times, uses an appropriate forward-looking phrase.

```html
{{ naturaltime .CreatedAt }}
{{ naturaltime .UpdatedAt }}
```

Examples (when now is April 1, 2026 16:30:00):

**Past:**

- `16:30:00` → `just now`
- `16:29:30` → `30 seconds ago`
- `16:29:00` → `1 minute ago`
- `16:25:00` → `5 minutes ago`
- `15:30:00` → `1 hour ago`
- `13:30:00` → `3 hours ago`
- `March 31 04:30` → `1 day ago`
- `March 22` → `1 week ago`
- `February 15` → `1 month ago`
- `April 1, 2025` → `1 year ago`

**Future:**

- `16:30:30` → `30 seconds from now`
- `16:35:00` → `5 minutes from now`
- `17:30:00` → `1 hour from now`

With German locale:

- `16:25:00` → `vor 5 Minuten`
- `16:35:00` → `in 5 Minuten`
- `16:30:00` → `gerade eben`

### `ordinal`

Converts an integer to its ordinal representation as a string.

Examples:

- `{{ ordinal 1 }}` → `1st`
- `{{ ordinal 2 }}` → `2nd`
- `{{ ordinal 3 }}` → `3rd`
- `{{ ordinal 4 }}` → `4th`
- `{{ ordinal 11 }}` → `11th`
- `{{ ordinal 12 }}` → `12th`
- `{{ ordinal 13 }}` → `13th`
- `{{ ordinal 21 }}` → `21st`
- `{{ ordinal 101 }}` → `101st`
- `{{ ordinal 111 }}` → `111th`

With German locale (always uses a period):

- `{{ ordinal 1 }}` → `1.`
- `{{ ordinal 2 }}` → `2.`
- `{{ ordinal 21 }}` → `21.`

### `filesizeformat`

Formats a byte count as a human-readable file size with locale-aware decimal separators.

Examples:

- `{{ filesizeformat 0 }}` → `0 bytes`
- `{{ filesizeformat 1 }}` → `1 byte`
- `{{ filesizeformat 1024 }}` → `1.0 KB`
- `{{ filesizeformat 1536 }}` → `1.5 KB`
- `{{ filesizeformat 13516 }}` → `13.2 KB`
- `{{ filesizeformat 1048576 }}` → `1.0 MB`
- `{{ filesizeformat 1073741824 }}` → `1.0 GB`
- `{{ filesizeformat 1099511627776 }}` → `1.0 TB`

With German locale:

- `{{ filesizeformat 1536 }}` → `1,5 KB`
- `{{ filesizeformat 13516 }}` → `13,2 KB`

Units: bytes, KB, MB, GB, TB, PB.

## Translations

Ships with English and German translations. Add your own by providing translation files following burrow's [i18n conventions](../guide/i18n.md). Translation keys use the `humanize-` prefix (e.g., `humanize-today`, `humanize-seconds-ago`).
