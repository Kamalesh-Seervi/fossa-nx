// Package email provides functionality for sending email notifications
package email

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"strings"
	"time"

	"github.com/jordan-wright/email"
	"github.com/kamalesh-seervi/fossa-nx/internal/models"
)

// HTML Email Template
const emailTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>FOSSA Security Report</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background-color: #0058a2;
            color: white;
            padding: 20px;
            text-align: center;
            border-radius: 5px 5px 0 0;
        }
        .content {
            padding: 20px;
            background-color: #f9f9f9;
            border-left: 1px solid #ddd;
            border-right: 1px solid #ddd;
        }
        .summary {
            margin-bottom: 30px;
            background-color: white;
            padding: 15px;
            border-radius: 5px;
            border-left: 5px solid #0058a2;
            box-shadow: 0 2px 5px rgba(0,0,0,0.1);
        }
        .summary-table {
            width: 100%;
            border-collapse: collapse;
        }
        .summary-table td, .summary-table th {
            padding: 8px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }
        .project {
            margin-bottom: 30px;
            background-color: white;
            padding: 15px;
            border-radius: 5px;
            box-shadow: 0 2px 5px rgba(0,0,0,0.1);
        }
        .project-header {
            border-bottom: 1px solid #eee;
            padding-bottom: 10px;
            margin-bottom: 15px;
            display: flex;
            justify-content: space-between;
        }
        .project-link {
            color: #0058a2;
            text-decoration: none;
        }
        .vulnerability {
            margin-bottom: 15px;
            padding: 15px;
            border-radius: 5px;
        }
        .high {
            border-left: 5px solid #d9534f;
            background-color: #ffeeee;
        }
        .medium {
            border-left: 5px solid #f0ad4e;
            background-color: #fff8ee;
        }
        .low {
            border-left: 5px solid #5bc0de;
            background-color: #f0f9ff;
        }
        .severity-badge {
            display: inline-block;
            padding: 5px 10px;
            border-radius: 3px;
            font-size: 12px;
            font-weight: bold;
            color: white;
        }
        .high-badge {
            background-color: #d9534f;
        }
        .medium-badge {
            background-color: #f0ad4e;
        }
        .low-badge {
            background-color: #5bc0de;
        }
        .footer {
            text-align: center;
            padding: 20px;
            font-size: 12px;
            color: #777;
            background-color: #f9f9f9;
            border-radius: 0 0 5px 5px;
            border: 1px solid #ddd;
        }
        .vuln-details {
            margin-top: 10px;
            font-size: 14px;
        }
        .vuln-cve {
            margin-top: 5px;
            font-family: monospace;
        }
        .vuln-meta {
            margin-top: 10px;
            font-size: 12px;
            color: #666;
        }
        .vuln-name {
            font-weight: bold;
            margin-bottom: 5px;
        }
        .no-issues {
            padding: 15px;
            background-color: #dff0d8;
            border-left: 5px solid #5cb85c;
            border-radius: 5px;
        }
        .stats-block {
            background-color: white;
            padding: 15px;
            border-radius: 5px;
            margin-bottom: 20px;
            box-shadow: 0 2px 5px rgba(0,0,0,0.1);
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>FOSSA Security Report</h1>
        <p>Generated on {{.Date}}</p>
    </div>
    <div class="content">
        <div class="summary">
            <h2>Summary</h2>
            <table class="summary-table">
                <tr>
                    <td>Total Projects Analyzed:</td>
                    <td>{{.TotalProjects}}</td>
                </tr>
                <tr>
                    <td>Successful Scans:</td>
                    <td>{{.SuccessfulProjects}}</td>
                </tr>
                <tr>
                    <td>Failed Scans:</td>
                    <td>{{.FailedProjects}}</td>
                </tr>
                <tr>
                    <td>Total Vulnerabilities:</td>
                    <td>{{.TotalVulnerabilities}}</td>
                </tr>
                <tr>
                    <td>High Severity:</td>
                    <td>{{.HighSeverity}}</td>
                </tr>
                <tr>
                    <td>Medium Severity:</td>
                    <td>{{.MediumSeverity}}</td>
                </tr>
                <tr>
                    <td>Low Severity:</td>
                    <td>{{.LowSeverity}}</td>
                </tr>
            </table>
        </div>

        <div class="stats-block">
            <h2>Scan Performance</h2>
            <table class="summary-table">
                <tr>
                    <td>Total Duration:</td>
                    <td>{{if gt .TotalDuration 60.0}}{{.TotalDurationMinutes}} minutes{{else}}{{.TotalDuration}} seconds{{end}}</td>
                </tr>
                <tr>
                    <td>Average Scan Time:</td>
                    <td>{{if gt .AverageDuration 60.0}}{{.AverageDurationMinutes}} minutes{{else}}{{.AverageDuration}} seconds{{end}}</td>
                </tr>
                <tr>
                    <td>Fastest Scan:</td>
                    <td>{{if gt .MinDuration 60.0}}{{.MinDurationMinutes}} minutes{{else}}{{.MinDuration}} seconds{{end}}</td>
                </tr>
                <tr>
                    <td>Slowest Scan:</td>
                    <td>{{if gt .MaxDuration 60.0}}{{.MaxDurationMinutes}} minutes{{else}}{{.MaxDuration}} seconds{{end}}</td>
                </tr>
            </table>
        </div>

        <h2>Vulnerabilities by Project</h2>

        {{if eq (len .ProjectsWithIssues) 0}}
            <div class="no-issues">
                <p>No vulnerabilities were detected across all projects! 🎉</p>
            </div>
        {{else}}
            {{range .ProjectsWithIssues}}
                <div class="project">
                    <div class="project-header">
                        <h3>{{.Project}}</h3>
                        <a href="{{.FossaLink}}" class="project-link" target="_blank">View in FOSSA</a>
                    </div>
                    
                    <p><strong>Dependencies:</strong> {{.DependencyCount}}</p>
                    <p><strong>Vulnerabilities:</strong> {{len .Issues}}</p>
                    
                    {{range .Issues}}
                        <div class="vulnerability {{.Severity | ToLower}}">
                            <div class="vuln-name">
                                {{.Name}}
                                <span class="severity-badge {{.Severity | ToLower}}-badge">{{.Severity}}</span>
                            </div>
                            <div class="vuln-details">
                                {{.Description}}
                            </div>
                            {{if .CVE}}
                                <div class="vuln-cve">
                                    CVE: {{.CVE}}
                                </div>
                            {{end}}
                            {{if .FixedIn}}
                                <div class="vuln-meta">
                                    <strong>Fixed in:</strong> {{.FixedIn}}
                                    <br>
                                    <strong>First seen:</strong> {{.FirstSeen.Format "Jan 2, 2006"}}
                                </div>
                            {{end}}
                            <div style="margin-top: 10px;">
                                <a href="{{.Link}}" target="_blank" style="color: #0058a2;">View details</a>
                            </div>
                        </div>
                    {{end}}
                </div>
            {{end}}
        {{end}}
    </div>
    <div class="footer">
        <p>This report was automatically generated by FOSSA-NX.<br>
        For questions or issues, please contact the Platform-Eng Team.</p>
    </div>
</body>
</html>
`

// Parse comma-separated email list into a slice of email addresses
func ParseEmailList(emails string) []string {
	rawList := strings.Split(emails, ",")
	result := make([]string, 0, len(rawList))

	for _, addr := range rawList {
		trimmed := strings.TrimSpace(addr)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// Template data structure for the email report
type TemplateData struct {
	Date                   string
	TotalProjects          int
	SuccessfulProjects     int
	FailedProjects         int
	TotalVulnerabilities   int
	HighSeverity           int
	MediumSeverity         int
	LowSeverity            int
	ProjectsWithIssues     []models.Result
	TotalDuration          float64
	AverageDuration        float64
	MinDuration            float64
	MaxDuration            float64
	TotalDurationMinutes   float64
	AverageDurationMinutes float64
	MinDurationMinutes     float64
	MaxDurationMinutes     float64
}

// SendHTMLReport sends an HTML email report of FOSSA scan results
func SendHTMLReport(results []models.Result, config models.EmailConfig, verbose bool) error {
	if !config.Enabled || len(config.ToEmails) == 0 {
		return nil
	}

	// Count vulnerabilities by severity
	var totalVulnerabilities, highSeverity, mediumSeverity, lowSeverity int
	var projectsWithIssues []models.Result
	var successfulProjects, failedProjects int

	for _, result := range results {
		if result.Error != nil {
			failedProjects++
			continue
		}

		successfulProjects++

		if len(result.Issues) > 0 {
			totalVulnerabilities += len(result.Issues)
			projectsWithIssues = append(projectsWithIssues, result)

			for _, issue := range result.Issues {
				switch strings.ToLower(issue.Severity) {
				case "high", "critical":
					highSeverity++
				case "medium", "moderate":
					mediumSeverity++
				case "low":
					lowSeverity++
				}
			}
		}
	}

	if totalVulnerabilities == 0 && verbose {
		log.Println("No vulnerabilities found, sending all-clear report")
	}

	// Calculate duration stats
	var totalDuration time.Duration
	minDuration := time.Hour * 24 // Initialize to a large value
	maxDuration := time.Duration(0)
	scanDuration := time.Duration(0) // The overall scan duration, not sum of individual scans

	// Record start time from the results
	var earliestStart time.Time
	var latestEnd time.Time

	// Find earliest start and latest end times to compute actual scan duration
	for i, result := range results {
		totalDuration += result.Duration

		if result.Duration < minDuration {
			minDuration = result.Duration
		}

		if result.Duration > maxDuration {
			maxDuration = result.Duration
		}

		// For the first result, initialize the earliest and latest times
		if i == 0 {
			earliestStart = result.EndTime.Add(-result.Duration)
			latestEnd = result.EndTime
		} else {
			resultStart := result.EndTime.Add(-result.Duration)
			if resultStart.Before(earliestStart) {
				earliestStart = resultStart
			}
			if result.EndTime.After(latestEnd) {
				latestEnd = result.EndTime
			}
		}
	}

	// Calculate overall scan duration if we have valid timestamps
	if !earliestStart.IsZero() && !latestEnd.IsZero() {
		scanDuration = latestEnd.Sub(earliestStart)
	}

	avgDuration := time.Duration(0)
	if len(results) > 0 {
		avgDuration = totalDuration / time.Duration(len(results))
	}

	// Format durations - use the total scan duration for TotalDuration
	// rather than sum of individual scans
	data := TemplateData{
		Date:                   time.Now().Format("January 2, 2006 at 15:04:05 MST"),
		TotalProjects:          len(results),
		SuccessfulProjects:     successfulProjects,
		FailedProjects:         failedProjects,
		TotalVulnerabilities:   totalVulnerabilities,
		HighSeverity:           highSeverity,
		MediumSeverity:         mediumSeverity,
		LowSeverity:            lowSeverity,
		ProjectsWithIssues:     projectsWithIssues,
		TotalDuration:          scanDuration.Seconds(),
		AverageDuration:        avgDuration.Seconds(),
		MinDuration:            minDuration.Seconds(),
		MaxDuration:            maxDuration.Seconds(),
		TotalDurationMinutes:   scanDuration.Minutes(),
		AverageDurationMinutes: avgDuration.Minutes(),
		MinDurationMinutes:     minDuration.Minutes(),
		MaxDurationMinutes:     maxDuration.Minutes(),
	}

	// Parse template with custom functions
	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	t, err := template.New("email").Funcs(funcMap).Parse(emailTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse email template: %v", err)
	}

	// Execute template to buffer
	var body bytes.Buffer
	if err := t.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to execute email template: %v", err)
	}

	// Create email
	e := email.NewEmail()
	e.From = config.FromEmail
	e.To = config.ToEmails

	// Set subject based on vulnerabilities found
	if totalVulnerabilities > 0 {
		e.Subject = fmt.Sprintf("FOSSA Security Report: %d Vulnerabilities Found (%d High, %d Medium, %d Low)",
			totalVulnerabilities, highSeverity, mediumSeverity, lowSeverity)
	} else {
		e.Subject = "FOSSA Security Report: No Vulnerabilities Detected"
	}

	e.HTML = body.Bytes()

	// Send email
	if verbose {
		log.Printf("Sending email report to %d recipients", len(config.ToEmails))
	}

	auth := smtp.PlainAuth("", config.SmtpUser, config.SmtpPassword, config.SmtpServer)
	err = e.Send(fmt.Sprintf("%s:%d", config.SmtpServer, config.SmtpPort), auth)
	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	if verbose {
		log.Println("Email report sent successfully")
	}

	return nil
}
