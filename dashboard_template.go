package main

import (
	"html/template"
	"net/http"

	"github.com/rs/zerolog/log"
)

const dashboardHTMLTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Redis Proxy Dashboard</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 0;
            padding: 0;
            background-color: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
        }
        header {
            background-color: #333;
            color: white;
            padding: 15px 0;
            text-align: center;
        }
        h1 {
            margin: 0;
        }
        nav {
            background-color: #444;
            padding: 10px 0;
            text-align: center;
        }
        nav a {
            color: white;
            text-decoration: none;
            margin: 0 15px;
            padding: 5px 10px;
            border-radius: 3px;
            transition: background-color 0.3s;
        }
        nav a:hover {
            background-color: #555;
        }
        .card {
            background-color: white;
            border-radius: 5px;
            padding: 20px;
            margin: 20px 0;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .card h2 {
            margin-top: 0;
            color: #333;
            border-bottom: 1px solid #eee;
            padding-bottom: 10px;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
            gap: 20px;
            margin-top: 20px;
        }
        .stat-card {
            background-color: white;
            border-radius: 5px;
            padding: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            text-align: center;
        }
        .stat-value {
            font-size: 2.5em;
            font-weight: bold;
            color: #3498db;
            margin: 10px 0;
        }
        .stat-label {
            color: #666;
            font-size: 0.9em;
        }
        .footer {
            text-align: center;
            padding: 20px;
            color: #666;
            font-size: 0.8em;
        }
    </style>
</head>
<body>
    <header>
        <h1>Redis Proxy Dashboard</h1>
    </header>
    
    <nav>
        <a href="/dashboard">Home</a>
        <a href="/dashboard/stats">Statistics</a>
        <a href="/dashboard/logs">Logs</a>
    </nav>
    
    <div class="container">
        <div class="card">
            <h2>Welcome to Redis Proxy Dashboard</h2>
            <p>This dashboard provides metrics and logs for the Redis HTTP Proxy system.</p>
            
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-label">Total Requests</div>
                    <div class="stat-value" id="total-requests">-</div>
                    <div class="stat-label">All time</div>
                </div>
                
                <div class="stat-card">
                    <div class="stat-label">Success Rate</div>
                    <div class="stat-value" id="success-rate">-</div>
                    <div class="stat-label">Last 24 hours</div>
                </div>
                
                <div class="stat-card">
                    <div class="stat-label">Avg Response Time</div>
                    <div class="stat-value" id="avg-response-time">-</div>
                    <div class="stat-label">Last hour (ms)</div>
                </div>
                
                <div class="stat-card">
                    <div class="stat-label">Active Topics</div>
                    <div class="stat-value" id="active-topics">-</div>
                    <div class="stat-label">Last 24 hours</div>
                </div>
            </div>
        </div>
        
        <div class="card">
            <h2>Getting Started</h2>
            <p>Use the navigation above to view detailed statistics and logs.</p>
            <ul>
                <li><strong>Statistics:</strong> View detailed metrics, request rates, response times, and more.</li>
                <li><strong>Logs:</strong> Browse detailed request and response logs.</li>
            </ul>
        </div>
    </div>
    
    <div class="footer">
        Redis HTTP Proxy Dashboard - Version 1.0
    </div>

    <script>
        // Fetch basic stats on page load
        document.addEventListener('DOMContentLoaded', function() {
            fetch('/dashboard/api/stats?period=all')
                .then(response => response.json())
                .then(data => {
                    document.getElementById('total-requests').textContent = data.total_requests || 0;
                })
                .catch(error => console.error('Error fetching all-time stats:', error));
                
            fetch('/dashboard/api/stats?period=day')
                .then(response => response.json())
                .then(data => {
                    const successRate = data.total_requests > 0 
                        ? Math.round((data.successful_requests / data.total_requests) * 100) 
                        : 0;
                    document.getElementById('success-rate').textContent = successRate + '%';
                    
                    // Count unique topics
                    const topicCount = Object.keys(data.requests_by_topic || {}).length;
                    document.getElementById('active-topics').textContent = topicCount;
                })
                .catch(error => console.error('Error fetching daily stats:', error));
                
            fetch('/dashboard/api/stats?period=hour')
                .then(response => response.json())
                .then(data => {
                    document.getElementById('avg-response-time').textContent = 
                        data.average_response_time ? Math.round(data.average_response_time) : 0;
                })
                .catch(error => console.error('Error fetching hourly stats:', error));
        });
    </script>
</body>
</html>
`

func renderDashboardTemplate(w http.ResponseWriter) {
	// Set content type
	w.Header().Set("Content-Type", "text/html")

	// Parse and execute template
	tmpl, err := template.New("dashboard").Parse(dashboardHTMLTemplate)
	if err != nil {
		log.Error().Err(err).Msg("Error parsing dashboard template")
		http.Error(w, "Error generating dashboard page", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, nil); err != nil {
		log.Error().Err(err).Msg("Error executing dashboard template")
	}
}
