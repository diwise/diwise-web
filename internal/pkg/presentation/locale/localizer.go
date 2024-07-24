package locale

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

type Bundle interface {
	For(acceptLanguage string) Localizer
}

type Localizer interface {
	Get(string) string
	GetWithData(string, map[string]any) string
}

type impl struct {
	bundle          *i18n.Bundle
	langs           map[string]struct{}
	defaultLanguage string
}

type wrapper struct {
	lang  string
	impl_ *impl
}

func NewLocalizer(assetPath string, languages ...string) Bundle {

	if len(languages) == 0 {
		panic("at least one language must be specified")
	}

	b := &impl{
		bundle:          i18n.NewBundle(language.English),
		langs:           map[string]struct{}{},
		defaultLanguage: languages[0],
	}

	b.bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	for _, lang := range languages {
		b.bundle.MustLoadMessageFile(fmt.Sprintf("%s/l10n/%s.toml", assetPath, lang))
		b.langs[lang] = struct{}{}
	}

	return b
}

func (l *impl) For(acceptLanguage string) Localizer {
	w := &wrapper{
		lang:  l.defaultLanguage,
		impl_: l,
	}

	accepted := strings.FieldsFunc(acceptLanguage, func(r rune) bool {
		return r == ',' || r == ';'
	})

	for _, lang := range accepted {
		if _, ok := l.langs[lang]; ok {
			w.lang = lang
			break
		}
	}

	return w
}

func (w *wrapper) Get(id string) string {
	return w.impl_.get(w.lang, id)
}

func (w *wrapper) GetWithData(id string, data map[string]any) string {
	return w.impl_.getWithData(w.lang, id, data)
}

func (l *impl) get(lang string, id string) string {
	localizer := i18n.NewLocalizer(l.bundle, lang)

	cfg := &i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    id,
			Other: id,
			One:   id,
		},
	}

	str, err := localizer.Localize(cfg)
	if err != nil {
		return id
	}
	return str
}

func (l *impl) getWithData(lang, id string, data map[string]any) string {
	localizer := i18n.NewLocalizer(l.bundle, lang)

	cfg := &i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    id,
			Other: id,
		},
		TemplateData: data,
	}
	str, err := localizer.Localize(cfg)
	if err != nil {
		return id
	}

	return str
}
