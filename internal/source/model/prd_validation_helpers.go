package model

import (
	"regexp"
	"strings"
)

func criterionNeedsClarification(criterion string) bool {
	trimmed := strings.TrimSpace(criterion)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if prdNegativeCaseRE.MatchString(lower) {
		return false
	}
	if prdWeakCriterionRE.MatchString(lower) {
		return true
	}
	return len(strings.Fields(trimmed)) < 4
}

func bundledDeliverySurfaces(story PRDStory) int {
	text := strings.ToLower(strings.Join(append(append([]string{story.Title, story.Description}, story.AcceptanceCriteria...), story.DependsOn...), " "))
	surfaces := 0
	for _, pattern := range []*regexp.Regexp{prdCLISurfaceRE, prdAPISurfaceRE, prdUISurfaceRE} {
		if pattern.MatchString(text) {
			surfaces++
		}
	}
	return surfaces
}
