package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiniAppBindingBootstrapPrecedesAnalyticsInjectionPoints(t *testing.T) {
	template, err := os.ReadFile("web/index.html")
	require.NoError(t, err)
	templateHTML := string(template)
	templateBootstrapIndex := strings.Index(templateHTML, miniAppBindingTicketBootstrapPlaceholder)
	templateUmamiIndex := strings.Index(templateHTML, "<!--umami-->")
	templateGoogleIndex := strings.Index(templateHTML, "<!--Google Analytics-->")
	require.GreaterOrEqual(t, templateBootstrapIndex, 0)
	require.GreaterOrEqual(t, templateUmamiIndex, 0)
	require.GreaterOrEqual(t, templateGoogleIndex, 0)
	assert.Less(t, templateBootstrapIndex, templateUmamiIndex)
	assert.Less(t, templateBootstrapIndex, templateGoogleIndex)

	originalIndexPage := indexPage
	indexPage = []byte("<head><!--miniapp-binding-bootstrap-->\n<!--umami-->\n<!--Google Analytics-->\n</head>")
	t.Cleanup(func() {
		indexPage = originalIndexPage
	})
	t.Setenv("UMAMI_WEBSITE_ID", "")
	t.Setenv("GOOGLE_ANALYTICS_ID", "")

	InjectMiniAppBindingTicketBootstrap()
	InjectUmamiAnalytics()
	InjectGoogleAnalytics()

	html := string(indexPage)
	bootstrapIndex := strings.Index(html, miniAppBindingTicketBootstrapWindowKey)
	umamiIndex := strings.Index(html, "<!--Umami QuantumNous-->")
	googleIndex := strings.Index(html, "<!--Google Analytics QuantumNous-->")

	require.GreaterOrEqual(t, bootstrapIndex, 0)
	require.GreaterOrEqual(t, umamiIndex, 0)
	require.GreaterOrEqual(t, googleIndex, 0)
	assert.Less(t, bootstrapIndex, umamiIndex)
	assert.Less(t, bootstrapIndex, googleIndex)
}
