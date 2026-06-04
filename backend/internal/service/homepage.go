package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

const KeyHomepageConfig = "homepage.config"

type SettingsHomepage struct {
	MetaTitle              string   `json:"meta_title"`
	MetaDescription        string   `json:"meta_description"`
	Keywords               []string `json:"keywords"`
	FaviconObjectID        string   `json:"favicon_object_id"`
	FaviconURL             string   `json:"favicon_url"`
	OGImageObjectID        string   `json:"og_image_object_id"`
	OGImageURL             string   `json:"og_image_url"`
	HeroEyebrow            string   `json:"hero_eyebrow"`
	HeroTitle              string   `json:"hero_title"`
	HeroDescription        string   `json:"hero_description"`
	HeroBackgroundObjectID string   `json:"hero_background_object_id"`
	HeroBackgroundURL      string   `json:"hero_background_url"`
	FeaturesTitle          string   `json:"features_title"`
	FeaturesDescription    string   `json:"features_description"`
	CTATitle               string          `json:"cta_title"`
	CTADescription         string          `json:"cta_description"`
	SchemaIncludeDefault   bool            `json:"schema_include_default"`
	SchemaCustom           json.RawMessage `json:"schema_custom"`
}

type SettingsHomepagePatch struct {
	MetaTitle              *string   `json:"meta_title,omitempty"`
	MetaDescription        *string   `json:"meta_description,omitempty"`
	Keywords               *[]string `json:"keywords,omitempty"`
	FaviconObjectID        *string   `json:"favicon_object_id,omitempty"`
	OGImageObjectID        *string   `json:"og_image_object_id,omitempty"`
	HeroEyebrow            *string   `json:"hero_eyebrow,omitempty"`
	HeroTitle              *string   `json:"hero_title,omitempty"`
	HeroDescription        *string   `json:"hero_description,omitempty"`
	HeroBackgroundObjectID *string   `json:"hero_background_object_id,omitempty"`
	FeaturesTitle          *string   `json:"features_title,omitempty"`
	FeaturesDescription    *string   `json:"features_description,omitempty"`
	CTATitle               *string   `json:"cta_title,omitempty"`
	CTADescription         *string          `json:"cta_description,omitempty"`
	SchemaIncludeDefault   *bool            `json:"schema_include_default,omitempty"`
	SchemaCustom           *json.RawMessage `json:"schema_custom,omitempty"`
}

func DefaultHomepageSettings() SettingsHomepage {
	return SettingsHomepage{
		MetaTitle:       "MediaHub — Self-hosted media & HLS streaming",
		MetaDescription: "MediaHub — nền tảng quản lý media tự host: file manager phân quyền, upload S3/MinIO, streaming HLS và Integration API cho CMS.",
		Keywords: []string{
			"media hub",
			"self-hosted",
			"HLS streaming",
			"file manager",
			"S3",
			"MinIO",
			"integration API",
		},
		HeroEyebrow: "Self-hosted media platform",
		HeroTitle:   "Quản lý media & streaming trên hạ tầng của bạn",
		HeroDescription: "MediaHub gom file manager, chuyển mã HLS và API tích hợp cho CMS — " +
			"monorepo Go + Next.js, sẵn sàng triển khai nội bộ hoặc cho khách hàng self-host.",
		FeaturesTitle:       "Tính năng chính",
		FeaturesDescription: "Một dashboard cho team vận hành, một API cho website tích hợp.",
		CTATitle:            "Tích hợp CMS trong vài phút",
		CTADescription:      "Tạo API key, gọi /api/v1 để upload, lấy delivery URL và embed video HLS.",
		SchemaIncludeDefault: true,
		SchemaCustom:         json.RawMessage("[]"),
	}
}

func homepageFromRaw(raw map[string]json.RawMessage) SettingsHomepage {
	def := DefaultHomepageSettings()
	v, ok := raw[KeyHomepageConfig]
	if !ok {
		return def
	}
	var hp SettingsHomepage
	if err := json.Unmarshal(v, &hp); err != nil {
		return def
	}
	return normalizeHomepage(hp)
}

func normalizeHomepage(hp SettingsHomepage) SettingsHomepage {
	hp.MetaTitle = strings.TrimSpace(hp.MetaTitle)
	hp.MetaDescription = strings.TrimSpace(hp.MetaDescription)
	hp.FaviconObjectID = strings.TrimSpace(hp.FaviconObjectID)
	hp.FaviconURL = strings.TrimSpace(hp.FaviconURL)
	hp.OGImageObjectID = strings.TrimSpace(hp.OGImageObjectID)
	hp.OGImageURL = strings.TrimSpace(hp.OGImageURL)
	hp.HeroEyebrow = strings.TrimSpace(hp.HeroEyebrow)
	hp.HeroTitle = strings.TrimSpace(hp.HeroTitle)
	hp.HeroDescription = strings.TrimSpace(hp.HeroDescription)
	hp.HeroBackgroundObjectID = strings.TrimSpace(hp.HeroBackgroundObjectID)
	hp.HeroBackgroundURL = strings.TrimSpace(hp.HeroBackgroundURL)
	hp.FeaturesTitle = strings.TrimSpace(hp.FeaturesTitle)
	hp.FeaturesDescription = strings.TrimSpace(hp.FeaturesDescription)
	hp.CTATitle = strings.TrimSpace(hp.CTATitle)
	hp.CTADescription = strings.TrimSpace(hp.CTADescription)
	hp.SchemaCustom = normalizeSchemaCustom(hp.SchemaCustom)
	if hp.Keywords == nil {
		hp.Keywords = []string{}
	}
	return hp
}

func (hp SettingsHomepage) validate() error {
	if hp.MetaTitle == "" || len(hp.MetaTitle) > 120 {
		return fmt.Errorf("%w: homepage.meta_title", ErrInvalidSettings)
	}
	if hp.MetaDescription == "" || len(hp.MetaDescription) > 320 {
		return fmt.Errorf("%w: homepage.meta_description", ErrInvalidSettings)
	}
	if len(hp.Keywords) > 30 {
		return fmt.Errorf("%w: homepage.keywords", ErrInvalidSettings)
	}
	for _, kw := range hp.Keywords {
		if len(kw) > 64 {
			return fmt.Errorf("%w: homepage.keywords", ErrInvalidSettings)
		}
	}
	if err := validateOptionalObjectID(hp.FaviconObjectID, "homepage.favicon_object_id"); err != nil {
		return err
	}
	if err := validateOptionalObjectID(hp.OGImageObjectID, "homepage.og_image_object_id"); err != nil {
		return err
	}
	if err := validateOptionalObjectID(hp.HeroBackgroundObjectID, "homepage.hero_background_object_id"); err != nil {
		return err
	}
	if hp.HeroTitle == "" || len(hp.HeroTitle) > 200 {
		return fmt.Errorf("%w: homepage.hero_title", ErrInvalidSettings)
	}
	if hp.HeroDescription == "" || len(hp.HeroDescription) > 600 {
		return fmt.Errorf("%w: homepage.hero_description", ErrInvalidSettings)
	}
	if len(hp.HeroEyebrow) > 120 {
		return fmt.Errorf("%w: homepage.hero_eyebrow", ErrInvalidSettings)
	}
	if hp.FeaturesTitle == "" || len(hp.FeaturesTitle) > 120 {
		return fmt.Errorf("%w: homepage.features_title", ErrInvalidSettings)
	}
	if hp.FeaturesDescription == "" || len(hp.FeaturesDescription) > 320 {
		return fmt.Errorf("%w: homepage.features_description", ErrInvalidSettings)
	}
	if hp.CTATitle == "" || len(hp.CTATitle) > 120 {
		return fmt.Errorf("%w: homepage.cta_title", ErrInvalidSettings)
	}
	if hp.CTADescription == "" || len(hp.CTADescription) > 320 {
		return fmt.Errorf("%w: homepage.cta_description", ErrInvalidSettings)
	}
	if err := validateSchemaCustom(hp.SchemaCustom); err != nil {
		return err
	}
	return nil
}

func validateOptionalObjectID(raw, field string) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	if _, err := uuid.Parse(s); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidSettings, field)
	}
	return nil
}

func validateOptionalURL(raw, field string) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "/") {
		if len(s) > 2048 {
			return fmt.Errorf("%w: %s", ErrInvalidSettings, field)
		}
		return nil
	}
	u, err := url.ParseRequestURI(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("%w: %s", ErrInvalidSettings, field)
	}
	return nil
}

func mergeHomepagePatch(current SettingsHomepage, patch SettingsHomepagePatch) SettingsHomepage {
	if patch.MetaTitle != nil {
		current.MetaTitle = strings.TrimSpace(*patch.MetaTitle)
	}
	if patch.MetaDescription != nil {
		current.MetaDescription = strings.TrimSpace(*patch.MetaDescription)
	}
	if patch.Keywords != nil {
		current.Keywords = normalizeKeywords(*patch.Keywords)
	}
	if patch.FaviconObjectID != nil {
		current.FaviconObjectID = strings.TrimSpace(*patch.FaviconObjectID)
		current.FaviconURL = ""
	}
	if patch.OGImageObjectID != nil {
		current.OGImageObjectID = strings.TrimSpace(*patch.OGImageObjectID)
		current.OGImageURL = ""
	}
	if patch.HeroEyebrow != nil {
		current.HeroEyebrow = strings.TrimSpace(*patch.HeroEyebrow)
	}
	if patch.HeroTitle != nil {
		current.HeroTitle = strings.TrimSpace(*patch.HeroTitle)
	}
	if patch.HeroDescription != nil {
		current.HeroDescription = strings.TrimSpace(*patch.HeroDescription)
	}
	if patch.HeroBackgroundObjectID != nil {
		current.HeroBackgroundObjectID = strings.TrimSpace(*patch.HeroBackgroundObjectID)
		current.HeroBackgroundURL = ""
	}
	if patch.FeaturesTitle != nil {
		current.FeaturesTitle = strings.TrimSpace(*patch.FeaturesTitle)
	}
	if patch.FeaturesDescription != nil {
		current.FeaturesDescription = strings.TrimSpace(*patch.FeaturesDescription)
	}
	if patch.CTATitle != nil {
		current.CTATitle = strings.TrimSpace(*patch.CTATitle)
	}
	if patch.CTADescription != nil {
		current.CTADescription = strings.TrimSpace(*patch.CTADescription)
	}
	if patch.SchemaIncludeDefault != nil {
		current.SchemaIncludeDefault = *patch.SchemaIncludeDefault
	}
	if patch.SchemaCustom != nil {
		current.SchemaCustom = normalizeSchemaCustom(*patch.SchemaCustom)
	}
	return normalizeHomepage(current)
}

func normalizeKeywords(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, kw := range in {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		key := strings.ToLower(kw)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, kw)
	}
	return out
}
