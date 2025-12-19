package aikit_test
//
// import (
// 	"os"
// 	"testing"
//
// 	"github.com/jacksonzamorano/aikit"
// )
//
// var xaiProvider = aikit.CompletionsAPI{
// 	Config: aikit.ProviderConfig{
// 		BaseURL: "https://api.x.ai",
// 		APIKey: os.Getenv("XAI_KEY"),
// 	},
// }
//
// func TestXAI(t *testing.T) {
// 	all := ""
// 	session, request := MakeRequest(xaiProvider, "grok-4-1-fast-reasoning", nil)
// 	result := session.Stream(request, func(result *aikit.ProviderState) {
// 		all += SnapshotResult(*result)
// 	})
// 	all += SnapshotResult(*result)
// 	VerifyResults(t, all, *result)
// }
