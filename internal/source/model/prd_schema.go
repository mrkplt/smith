package model

type PRD struct {
	Version   int         `json:"version"`
	Project   string      `json:"project"`
	Overview  string      `json:"overview"`
	Goals     []string    `json:"goals"`
	NonGoals  []string    `json:"nonGoals"`
	Success   []string    `json:"successMetrics"`
	Questions []string    `json:"openQuestions"`
	Stack     PRDStack    `json:"stack"`
	Routes    []PRDRoute  `json:"routes"`
	UI        string      `json:"uiNotes"`
	DataModel []PRDEntity `json:"dataModel"`
	Import    string      `json:"importFormat"`
	Rules     []string    `json:"rules"`
	Gates     []string    `json:"qualityGates"`
	Stories   []PRDStory  `json:"stories"`
}

type PRDStack struct {
	Framework string `json:"framework"`
	Hosting   string `json:"hosting"`
	Database  string `json:"database"`
	Auth      string `json:"auth"`
}

type PRDRoute struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
}

type PRDEntity struct {
	Name   string     `json:"name"`
	Fields []PRDField `json:"fields"`
}

type PRDField struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Note string `json:"note,omitempty"`
}

type PRDStory struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Status             string   `json:"status"` // always "open" initially
	DependsOn          []string `json:"dependsOn,omitempty"`
	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
}

type PRDValidationReport struct {
	Valid     bool                      `json:"valid"`
	Errors    []PRDValidationDiagnostic `json:"errors,omitempty"`
	Warnings  []PRDValidationDiagnostic `json:"warnings,omitempty"`
	Readiness string                    `json:"readiness"`
}

type PRDValidationDiagnostic struct {
	Code       string `json:"code"`
	Path       string `json:"path"`
	StoryID    string `json:"storyId,omitempty"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}
