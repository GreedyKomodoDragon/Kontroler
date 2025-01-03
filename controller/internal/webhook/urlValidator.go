package webhook

type UrlValidator interface {
	ValidateUrl(url string) error
}
