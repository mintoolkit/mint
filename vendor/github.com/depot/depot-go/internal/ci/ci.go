package ci

import (
	"os"
	"strings"
)

// Provider uses environment variables heuristics to determine if running in CI.
// Inspired by  https://github.com/watson/ci-info/blob/master/vendors.json
//
// Returns the name of the CI provider and a boolean indicating if it is a CI environment.
func Provider() (string, bool) {
	providers := map[string]string{
		"AC_APPCIRCLE":                       "Appcircle",
		"APPVEYOR":                           "AppVeyor",
		"CODEBUILD_BUILD_ARN":                "AWS CodeBuild",
		"SYSTEM_TEAMFOUNDATIONCOLLECTIONURI": "Azure Pipelines",
		"bamboo_planKey":                     "Bamboo",
		"BITBUCKET_COMMIT":                   "Bitbucket Pipelines",
		"BITRISE_IO":                         "Bitrise",
		"BUDDY_WORKSPACE_ID":                 "Buddy",
		"BUILDKITE":                          "Buildkite",
		"CIRCLECI":                           "CircleCI",
		"CIRRUS_CI":                          "Cirrus CI",
		"CF_BUILD_ID":                        "Codefresh",
		"CM_BUILD_ID":                        "Codemagic",
		"DRONE":                              "Drone",
		"DSARI":                              "dsari",
		"EAS_BUILD":                          "Expo Application Services",
		"GERRIT_PROJECT":                     "Gerrit",
		"GITHUB_ACTIONS":                     "GitHub Actions",
		"GITLAB_CI":                          "GitLab CI",
		"GO_PIPELINE_LABEL":                  "GoCD",
		"BUILDER_OUTPUT":                     "Google Cloud Build",
		"HARNESS_BUILD_ID":                   "Harness CI",
		"HUDSON_URL":                         "Hudson",
		"LAYERCI":                            "LayerCI",
		"MAGNUM":                             "Magnum CI",
		"NETLIFY":                            "Netlify",
		"NEVERCODE":                          "Nevercode",
		"RELEASEHUB":                         "Release Hub",
		"RENDER":                             "Render",
		"SAILCI":                             "Sail CI",
		"SCREWDRIVER":                        "Screwdriver",
		"SEMAPHORE":                          "Semaphore",
		"SHIPPABLE":                          "Shippable",
		"TDDIUM":                             "Solano",
		"STRIDER":                            "Strider CI",
		"TEAMCITY_VERSION":                   "TeamCity",
		"TRAVIS":                             "Travis CI",
		"NOW_BUILDER":                        "Vercel",
		"VERCEL":                             "Vercel",
		"APPCENTER_BUILD_ID":                 "Visual Studio App Center",
		"CI_XCODE_PROJECT":                   "Xcode Cloud",
		"XCS":                                "Xcode Server",
	}

	env := map[string]string{}

	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		env[(pair[0])] = pair[1]
	}

	for envKey, provider := range providers {
		if _, ok := env[envKey]; ok {
			return provider, true
		}
	}

	// Heroku is more complicated. This is what some of the CI detectors are using.
	if node, ok := env["NODE"]; ok {
		if strings.Contains(node, "/app/.heroku/node/bin/node") {
			return "Heroku", true
		}
	}

	if ciName, ok := env["CI_NAME"]; ok {
		switch ciName {
		case "codeship":
			return "Codeship", true
		case "sourcehut":
			return "SourceHut", true
		}
	}

	if ciName, ok := env["CI"]; ok {
		switch ciName {
		case "woodpecker":
			return "Woodpecker", true
		case "1":
		case "true":
			return "Generic CI", true
		}
	}

	if _, ok := env["TASK_ID"]; ok {
		if _, ok := env["RUN_ID"]; ok {
			return "TaskCluster", true
		}
	}

	if _, ok := env["JENKINS_URL"]; ok {
		if _, ok := env["BUILD_ID"]; ok {
			return "Jenkins", true
		}
	}

	return "", false
}
