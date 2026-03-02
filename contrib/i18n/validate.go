package i18n

import (
	"context"

	"codeberg.org/oliverandrich/burrow"
)

// TranslateValidationErrors translates the messages of a ValidationError
// using the localizer from the context. If no localizer is present or a
// translation key is missing, the original English message is preserved.
func TranslateValidationErrors(ctx context.Context, ve *burrow.ValidationError) {
	for i := range ve.Errors {
		fe := &ve.Errors[i]
		key := "validation-" + fe.Tag
		data := map[string]any{"Field": fe.Field, "Param": fe.Param}
		translated := TData(ctx, key, data)
		if translated != key {
			fe.Message = translated
		}
	}
}
