package i18n

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"zh_CN.UTF-8": Chinese,
		"zh_TW.UTF-8": Chinese,
		"zh-Hans-CN":  Chinese,
		"zh":          Chinese,
		"en_US.UTF-8": English,
		"en":          English,
		"C":           English,
		"POSIX":       English,
		"":            English,
	}

	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFor(t *testing.T) {
	if got := For("zh_CN.UTF-8").TodosTitle; got != "待办" {
		t.Errorf("Chinese TodosTitle = %q", got)
	}
	if got := For("en_US.UTF-8").TodosTitle; got != "Todos" {
		t.Errorf("English TodosTitle = %q", got)
	}
	if got := For("").TodosTitle; got == "" {
		t.Error("an empty language should still resolve to a set")
	}
}

// TestDetectPrefersLCALL pins the precedence libc uses, so a stray LANG can't
// override an explicit LC_ALL.
func TestDetectPrefersLCALL(t *testing.T) {
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	t.Setenv("LANG", "en_US.UTF-8")
	if got := Detect(); got != Chinese {
		t.Errorf("Detect() = %q, want %q", got, Chinese)
	}

	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANGUAGE"} {
		t.Setenv(key, "")
	}
	t.Setenv("LANG", "en_US.UTF-8")
	if got := Detect(); got != English {
		t.Errorf("Detect() = %q, want %q", got, English)
	}
}

// TestNoMissingTranslations fails when a set gains a field but only one language
// fills it in.
func TestNoMissingTranslations(t *testing.T) {
	for name, set := range map[string]*TranslationSet{
		"english": EnglishSet(),
		"chinese": ChineseSet(),
	} {
		value := reflect.ValueOf(*set)
		for i := 0; i < value.NumField(); i++ {
			if strings.TrimSpace(value.Field(i).String()) == "" {
				t.Errorf("%s: field %s is empty", name, value.Type().Field(i).Name)
			}
		}
	}
}

// TestFormatStringsAgree keeps the two languages from drifting apart on the
// number of verbs in a format string.
func TestFormatStringsAgree(t *testing.T) {
	en, zh := reflect.ValueOf(*EnglishSet()), reflect.ValueOf(*ChineseSet())

	for i := 0; i < en.NumField(); i++ {
		name := en.Type().Field(i).Name
		want := strings.Count(en.Field(i).String(), "%")
		if got := strings.Count(zh.Field(i).String(), "%"); got != want {
			t.Errorf("%s: english has %d verbs, chinese has %d (%q)", name, want, got, zh.Field(i).String())
		}
	}
}

var _ = fmt.Sprintf // keep the import list stable if the checks above change
