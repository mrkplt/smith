package model

import (
	"encoding/json"
	"errors"
	"fmt"
)

func (p *PRD) Validate() error {
	report := p.ValidateReport()
	if report.Valid {
		return nil
	}
	return errors.New(report.Errors[0].Message)
}

func ParsePRDJSON(data []byte) (*PRD, error) {
	var prd PRD
	if err := json.Unmarshal(data, &prd); err != nil {
		return nil, err
	}
	return &prd, nil
}

func ValidatePRDJSON(data []byte) (*PRD, PRDValidationReport) {
	prd, err := ParsePRDJSON(data)
	if err != nil {
		return nil, PRDValidationReport{
			Valid: false,
			Errors: []PRDValidationDiagnostic{
				{
					Code:       PRDDiagnosticMalformedJSON,
					Path:       "$",
					Message:    fmt.Sprintf("PRD JSON could not be parsed: %v", err),
					Suggestion: "Fix the JSON syntax and try validation again.",
				},
			},
			Readiness: PRDReadinessFail,
		}
	}
	return prd, prd.ValidateReport()
}
