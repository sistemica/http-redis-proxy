package main

import (
	"html/template"
	"net/http"

	"github.com/rs/zerolog/log"
)

const statsHTMLTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Redis Proxy Statistics</title>
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
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .card h2 {
            margin-top: 0;
            color: #333;
            border-bottom: 1px solid #eee;
            padding-bottom: 10px;
        }
        .controls {
            display: flex;
            align-items: center;
            margin-bottom: 20px;
            padding: 15px;
            background-color: white;
            border-radius: 5px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
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
        button {
            padding: 8px 15px;
            background-color: #4285f4;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
        }
        button:hover {
            background-color: #3367d6;
        }
        select {
            padding: 8px;
            border: 1px solid #ddd;
            border-radius: 4px;
            margin-right: 10px;
        }
        #auto-refresh-container {
            margin-left: 20px;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }
        th, td {
            padding: 10px;
            border: 1px solid #ddd;
            text-align: left;
        }
        th {
            background-color: #f2f2f2;
            font-weight: bold;
        }
        tr:nth-child(even) {
            background-color: #f9f9f9;
        }
        .footer {
            text-align: center;
            padding: 20px;
            color: #666;
            font-size: 0.8em;
        }
        .chart-container {
            height: 300px;
            margin-top: 20px;
        }
        .loading {
            text-align: center;
            padding: 40px;
            color: #666;
        }
        .error {
            color: #f44336;
            font-weight: bold;
            text-align: center;
            padding: 20px;
        }
    </style>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@3.7.1/dist/chart.min.js"></script>
</head>
<body>
    <header>
        <h1>Redis Proxy Statistics</h1>
    </header>
    
    <nav>
        <a href="/dashboard">Home</a>
        <a href="/dashboard/stats">Statistics</a>
        <a href="/dashboard/logs">Logs</a>
    </nav>
    
    <div class="container">
        <div class="controls">
            <div>
                <label for="period">Time Period:</label>
                <select id="period">
                    <option value="hour">Last Hour</option>
                    <option value="day" selected>Last 24 Hours</option>
                    <option value="week">Last 7 Days</option>
                    <option value="month">Last 30 Days</option>
                    <option value="all">All Time</option>
                </select>
            </div>
            
            <button id="refresh-btn">Refresh</button>
            
            <div id="auto-refresh-container">
                <input type="checkbox" id="auto-refresh" name="auto-refresh">
                <label for="auto-refresh">Auto refresh</label>
                <select id="refresh-interval">
                    <option value="30">30 seconds</option>
                    <option value="60">1 minute</option>
                    <option value="300">5 minutes</option>
                </select>
            </div>
        </div>
        
        <div id="stats-container">
            <div class="loading">Loading statistics...</div>
        </div>
    </div>
    
    <div class="footer">
        Redis HTTP Proxy Statistics - Version 1.0
    </div>

    <script>
        let autoRefreshInterval;
        let statusChart;
        let topicsChart;
        
        // Function to format numbers with commas
        function formatNumber(num) {
            return num.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
        }
        
        // Function to format milliseconds
        function formatTime(ms) {
            if (ms < 1) return "< 1 ms";
            if (ms < 1000) return Math.round(ms) + " ms";
            return (ms / 1000).toFixed(2) + " sec";
        }
        
        // Function to create or update charts
        function updateCharts(data) {
            // Status code chart
            const statusCodes = Object.keys(data.requests_by_status_code || {});
            const statusCounts = statusCodes.map(code => data.requests_by_status_code[code]);
            
            const statusColors = statusCodes.map(code => {
                if (code >= 200 && code < 300) return '#4caf50';
                if (code >= 400 && code < 500) return '#ff9800';
                if (code >= 500) return '#f44336';
                return '#9e9e9e';
            });
            
            if (statusChart) {
                statusChart.data.labels = statusCodes;
                statusChart.data.datasets[0].data = statusCounts;
                statusChart.data.datasets[0].backgroundColor = statusColors;
                statusChart.update();
            } else if (statusCodes.length > 0) {
                const ctx = document.getElementById('status-chart');
                statusChart = new Chart(ctx, {
                    type: 'bar',
                    data: {
                        labels: statusCodes,
                        datasets: [{
                            label: 'Requests by Status Code',
                            data: statusCounts,
                            backgroundColor: statusColors
                        }]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: {
                            legend: {
                                display: false
                            },
                            title: {
                                display: true,
                                text: 'Requests by Status Code'
                            }
                        },
                        scales: {
                            y: {
                                beginAtZero: true
                            }
                        }
                    }
                });
            }
            
            // Topics chart
            const topics = Object.keys(data.requests_by_topic || {});
            const topicCounts = topics.map(topic => data.requests_by_topic[topic]);
            
            if (topicsChart) {
                topicsChart.data.labels = topics;
                topicsChart.data.datasets[0].data = topicCounts;
                topicsChart.update();
            } else if (topics.length > 0) {
                const ctx = document.getElementById('topics-chart');
                topicsChart = new Chart(ctx, {
                    type: 'pie',
                    data: {
                        labels: topics,
                        datasets: [{
                            data: topicCounts,
                            backgroundColor: [
                                '#4285f4', '#ea4335', '#fbbc05', '#34a853', 
                                '#8e44ad', '#2c3e50', '#f39c12', '#d35400',
                                '#c0392b', '#16a085', '#27ae60', '#2980b9'
                            ]
                        }]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: {
                            legend: {
                                position: 'right'
                            },
                            title: {
                                display: true,
                                text: 'Requests by Topic'
                            }
                        }
                    }
                });
            }
        }
        
        // Function to fetch and display statistics
        function fetchStats() {
            const period = document.getElementById('period').value;
            const statsContainer = document.getElementById('stats-container');
            
            statsContainer.innerHTML = '<div class="loading">Loading statistics...</div>';
            
            fetch('/dashboard/api/stats?period=' + period + '&window_size=10')
                .then(response => {
                    if (!response.ok) {
                        throw new Error('Failed to fetch statistics');
                    }
                    return response.json();
                })
                .then(data => {
                    // Build statistics UI
                    let content = '';
                    
                    // Overview section
                    content += '<div class="card">';
                    content += '<h2>Overview</h2>';
                    content += '<div class="stats-grid">';
                    
                    content += '<div class="stat-card">';
                    content += '<div class="stat-label">Total Requests</div>';
                    content += '<div class="stat-value">' + formatNumber(data.total_requests || 0) + '</div>';
                    content += '<div class="stat-label">' + getPeriodLabel(period) + '</div>';
                    content += '</div>';
                    
                    content += '<div class="stat-card">';
                    content += '<div class="stat-label">Success Rate</div>';
                    const successRate = data.total_requests > 0 
                        ? Math.round((data.successful_requests / data.total_requests) * 100) 
                        : 0;
                    content += '<div class="stat-value">' + successRate + '%</div>';
                    content += '<div class="stat-label">' + getPeriodLabel(period) + '</div>';
                    content += '</div>';
                    
                    content += '<div class="stat-card">';
                    content += '<div class="stat-label">Failed Requests</div>';
                    content += '<div class="stat-value">' + formatNumber(data.failed_requests || 0) + '</div>';
                    content += '<div class="stat-label">' + getPeriodLabel(period) + '</div>';
                    content += '</div>';
                    
                    content += '<div class="stat-card">';
                    content += '<div class="stat-label">Timeouts</div>';
                    content += '<div class="stat-value">' + formatNumber(data.timeout_requests || 0) + '</div>';
                    content += '<div class="stat-label">' + getPeriodLabel(period) + '</div>';
                    content += '</div>';
                    
                    content += '</div>'; // End of stats-grid
                    content += '</div>'; // End of overview card
                    
                    // Performance section
                    content += '<div class="card">';
                    content += '<h2>Performance</h2>';
                    content += '<div class="stats-grid">';
                    
                    content += '<div class="stat-card">';
                    content += '<div class="stat-label">Average Response Time</div>';
                    content += '<div class="stat-value">' + (data.average_response_time ? Math.round(data.average_response_time) : 0) + ' ms</div>';
                    content += '<div class="stat-label">' + getPeriodLabel(period) + '</div>';
                    content += '</div>';
                    
                    content += '<div class="stat-card">';
                    content += '<div class="stat-label">Min Response Time</div>';
                    content += '<div class="stat-value">' + (data.min_response_time || 0) + ' ms</div>';
                    content += '<div class="stat-label">' + getPeriodLabel(period) + '</div>';
                    content += '</div>';
                    
                    content += '<div class="stat-card">';
                    content += '<div class="stat-label">Max Response Time</div>';
                    content += '<div class="stat-value">' + (data.max_response_time || 0) + ' ms</div>';
                    content += '<div class="stat-label">' + getPeriodLabel(period) + '</div>';
                    content += '</div>';
                    
                    content += '</div>'; // End of stats-grid
                    content += '</div>'; // End of performance card
                    
                    // Charts section
                    content += '<div class="card">';
                    content += '<h2>Request Distribution</h2>';
                    content += '<div style="display: flex; flex-wrap: wrap;">';
                    
                    content += '<div style="flex: 1; min-width: 400px;">';
                    content += '<div class="chart-container"><canvas id="status-chart"></canvas></div>';
                    content += '</div>';
                    
                    content += '<div style="flex: 1; min-width: 400px;">';
                    content += '<div class="chart-container"><canvas id="topics-chart"></canvas></div>';
                    content += '</div>';
                    
                    content += '</div>'; // End of flex container
                    content += '</div>'; // End of charts card
                    
                    // Status code details table
                    content += '<div class="card">';
                    content += '<h2>Status Code Distribution</h2>';
                    content += '<table>';
                    content += '<thead><tr><th>Status Code</th><th>Count</th><th>Percentage</th></tr></thead>';
                    content += '<tbody>';
                    
                    const statusCodes = Object.keys(data.requests_by_status_code || {}).sort((a, b) => a - b);
                    statusCodes.forEach(code => {
                        const count = data.requests_by_status_code[code];
                        const percentage = data.total_requests > 0 
                            ? ((count / data.total_requests) * 100).toFixed(1) + '%' 
                            : '0%';
                        content += '<tr>';
                        content += '<td>' + code + '</td>';
                        content += '<td>' + count + '</td>';
                        content += '<td>' + percentage + '</td>';
                        content += '</tr>';
                    });
                    
                    content += '</tbody>';
                    content += '</table>';
                    content += '</div>'; // End of status code card
                    
                    // Topics details table
                    content += '<div class="card">';
                    content += '<h2>Top Topics</h2>';
                    content += '<table>';
                    content += '<thead><tr><th>Topic</th><th>Count</th><th>Percentage</th></tr></thead>';
                    content += '<tbody>';
                    
                    const topics = Object.keys(data.requests_by_topic || {});
                    topics.forEach(topic => {
                        const count = data.requests_by_topic[topic];
                        const percentage = data.total_requests > 0 
                            ? ((count / data.total_requests) * 100).toFixed(1) + '%' 
                            : '0%';
                        content += '<tr>';
                        content += '<td>' + topic + '</td>';
                        content += '<td>' + count + '</td>';
                        content += '<td>' + percentage + '</td>';
                        content += '</tr>';
                    });
                    
                    content += '</tbody>';
                    content += '</table>';
                    content += '</div>'; // End of topics card
                    
                    // Update the container
                    statsContainer.innerHTML = content;
                    
                    // Create charts after the DOM is updated
                    updateCharts(data);
                })
                .catch(error => {
                    statsContainer.innerHTML = '<div class="error">Error: ' + error.message + '</div>';
                });
        }
        
        // Helper function to get period label
        function getPeriodLabel(period) {
            switch(period) {
                case 'hour': return 'Last Hour';
                case 'day': return 'Last 24 Hours';
                case 'week': return 'Last 7 Days';
                case 'month': return 'Last 30 Days';
                case 'all': return 'All Time';
                default: return period;
            }
        }

        // Set up event listeners
        document.addEventListener('DOMContentLoaded', function() {
            // Initial fetch
            fetchStats();
            
            // Refresh button
            document.getElementById('refresh-btn').addEventListener('click', fetchStats);
            
            // Period change
            document.getElementById('period').addEventListener('change', fetchStats);
            
            // Auto refresh
            const autoRefreshCheckbox = document.getElementById('auto-refresh');
            const refreshIntervalSelect = document.getElementById('refresh-interval');
            
            function updateAutoRefresh() {
                clearInterval(autoRefreshInterval);
                
                if (autoRefreshCheckbox.checked) {
                    const interval = parseInt(refreshIntervalSelect.value) * 1000;
                    autoRefreshInterval = setInterval(fetchStats, interval);
                }
            }
            
            autoRefreshCheckbox.addEventListener('change', updateAutoRefresh);
            refreshIntervalSelect.addEventListener('change', updateAutoRefresh);
        });
    </script>
</body>
</html>
`

func renderStatsTemplate(w http.ResponseWriter) {
	// Set content type
	w.Header().Set("Content-Type", "text/html")

	// Parse and execute template
	tmpl, err := template.New("stats").Parse(statsHTMLTemplate)
	if err != nil {
		log.Error().Err(err).Msg("Error parsing stats template")
		http.Error(w, "Error generating stats page", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, nil); err != nil {
		log.Error().Err(err).Msg("Error executing stats template")
	}
}
