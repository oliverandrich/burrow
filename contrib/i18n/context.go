// Package i18n provides internationalization as a burrow contrib app.
//
// The middleware detects the user's locale from the Accept-Language header
// and stores a localizer in the request context.
//
// Use [T] for simple message translation, [TData] for messages with template
// data, and [TPlural] for messages with plural support. All three fall back
// to the message key if no localizer is present.
//
// [Locale] returns the current locale string, defaulting to "en".
package i18n

import (
	"context"

	i18nlib "github.com/nicksnyder/go-i18n/v2/i18n"
)

type (
	ctxKeyLocalizer struct{}
	ctxKeyLocale    struct{}
)

// T translates a message by ID using the localizer from context.
// Falls back to the message ID if no localizer is present.
func T(ctx context.Context, key string) string {
	localizer, ok := ctx.Value(ctxKeyLocalizer{}).(*i18nlib.Localizer)
	if !ok {
		return key
	}
	msg, err := localizer.Localize(&i18nlib.LocalizeConfig{MessageID: key})
	if err != nil {
		return key
	}
	return msg
}

// TData translates a message with template data using the localizer from context.
// Falls back to the message ID if no localizer is present.
func TData(ctx context.Context, key string, data map[string]any) string {
	localizer, ok := ctx.Value(ctxKeyLocalizer{}).(*i18nlib.Localizer)
	if !ok {
		return key
	}
	msg, err := localizer.Localize(&i18nlib.LocalizeConfig{
		MessageID:    key,
		TemplateData: data,
	})
	if err != nil {
		return key
	}
	return msg
}

// TPlural translates a message with plural support using the localizer from context.
// Falls back to the message ID if no localizer is present.
func TPlural(ctx context.Context, key string, count int) string {
	localizer, ok := ctx.Value(ctxKeyLocalizer{}).(*i18nlib.Localizer)
	if !ok {
		return key
	}
	msg, err := localizer.Localize(&i18nlib.LocalizeConfig{
		MessageID:    key,
		PluralCount:  count,
		TemplateData: map[string]any{"Count": count},
	})
	if err != nil {
		return key
	}
	return msg
}

// Locale returns the current locale from context, defaulting to "en".
func Locale(ctx context.Context) string {
	if locale, ok := ctx.Value(ctxKeyLocale{}).(string); ok {
		return locale
	}
	return "en"
}
