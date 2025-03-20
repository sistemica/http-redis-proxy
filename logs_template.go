package main

import (
	"html/template"
	"net/http"

	"github.com/rs/zerolog/log"
)

const logsHTMLTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Redis Proxy Logs</title>
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
        .controls {
            margin-bottom: 20px;
            display: flex;
            align-items: center;
            gap: 10px;
            background-color: white;
            padding: 15px;
            border-radius: 5px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .log-entry {
            background-color: white;
            border-radius: 5px;
            padding: 15px;
            margin-bottom: 15px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .log-header {
            display: flex;
            justify-content: space-between;
            border-bottom: 1px solid #eee;
            padding-bottom: 10px;
            margin-bottom: 10px;
        }
        .request-info {
            font-weight: bold;
        }
        .status-code {
            padding: 3px 8px;
            border-radius: 3px;
            color: white;
        }
        .status-2xx {
            background-color: #4caf50;
        }
        .status-4xx {
            background-color: #ff9800;
        }
        .status-5xx {
            background-color: #f44336;
        }
        .status-0 {
            background-color: #9e9e9e;
        }
        .details {
            display: flex;
            flex-wrap: wrap;
            gap: 10px;
        }
        .detail-item {
            flex: 1;
            min-width: 200px;
        }
        pre {
            background-color: #f8f8f8;
            border: 1px solid #ddd;
            border-radius: 3px;
            padding: 10px;
            max-height: 300px;
            overflow: auto;
            white-space: pre-wrap;
        }
        .empty-message {
            text-align: center;
            padding: 40px;
            color: #666;
        }
        .loading {
            text-align: center;
            padding: 20px;
            color: #666;
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
        input, select {
            padding: 8px;
            border: 1px solid #ddd;
            border-radius: 4px;
        }
        .error {
            color: #f44336;
            font-weight: bold;
        }
        .response-time {
            color: #666;
        }
        .response-time-slow {
            color: #ff9800;
        }
        .response-time-very-slow {
            color: #f44336;
        }
        .timestamp {
            color: #666;
            font-size: 0.9em;
        }
        #auto-refresh-container {
            margin-left: 20px;
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
        <h1>Redis Proxy Logs</h1>
    </header>
    
    <nav>
        <a href="/dashboard">Home</a>
        <a href="/dashboard/stats">Statistics</a>
        <a href="/dashboard/logs">Logs</a>
    </nav>
    
    <div class="container">
        <div class="controls">
            <button id="refresh-btn">Refresh</button>
            
            <div id="auto-refresh-container">
                <input type="checkbox" id="auto-refresh" name="auto-refresh">
                <label for="auto-refresh">Auto refresh</label>
                <select id="refresh-interval">
                    <option value="5">5 seconds</option>
                    <option value="10">10 seconds</option>
                    <option value="30">30 seconds</option>
                    <option value="60">1 minute</option>
                </select>
            </div>
            
            <div style="margin-left: auto;">
                <label for="limit">Show:</label>
                <select id="limit" name="limit">
                    <option value="10">10 entries</option>
                    <option value="25" selected>25 entries</option>
                    <option value="50">50 entries</option>
                    <option value="100">100 entries</option>
                    <option value="500">500 entries</option>
                </select>
            </div>
        </div>
        
        <div id="logs-container">
            <div class="loading">Loading logs...</div>
        </div>
    </div>
    
    <div class="footer">
        Redis HTTP Proxy Logs - Version 1.0
    </div>

    <script>
        let autoRefreshInterval;
        
        // Function to format JSON
        function formatJSON(json) {
            try {
                if (typeof json === 'string') {
                    if (!json) return '';
                    const obj = JSON.parse(json);
                    return JSON.stringify(obj, null, 2);
                } else {
                    return JSON.stringify(json, null, 2);
                }
            } catch (e) {
                return json; // Return as is if not valid JSON
            }
        }
        
        // Function to format timestamp
        function formatTimestamp(timestamp) {
            if (!timestamp) return '';
            const date = new Date(timestamp);
            return date.toLocaleString();
        }
        
        // Function to get status code class
        function getStatusCodeClass(code) {
            if (!code) return 'status-0';
            if (code >= 200 && code < 300) return 'status-2xx';
            if (code >= 400 && code < 500) return 'status-4xx';
            if (code >= 500) return 'status-5xx';
            return 'status-0';
        }
        
        // Function to get response time class
        function getResponseTimeClass(time) {
            if (!time) return '';
            if (time > 1000) return 'response-time-very-slow';
            if (time > 500) return 'response-time-slow';
            return '';
        }

        // Function to fetch and display logs
        function fetchLogs() {
            const limit = document.getElementById('limit').value;
            const logsContainer = document.getElementById('logs-container');
            
            logsContainer.innerHTML = '<div class="loading">Loading logs...</div>';
            
            fetch('/dashboard/api/logs?limit=' + limit)
                .then(response => {
                    if (!response.ok) {
                        throw new Error('Failed to fetch logs');
                    }
                    return response.json();
                })
                .then(logs => {
                    if (!logs || logs.length === 0) {
                        logsContainer.innerHTML = '<div class="empty-message">No logs found</div>';
                        return;
                    }
                    
                    logsContainer.innerHTML = '';
                    
                    logs.forEach(log => {
                        const logEntry = document.createElement('div');
                        logEntry.className = 'log-entry';
                        
                        // Create log header
                        const logHeader = document.createElement('div');
                        logHeader.className = 'log-header';
                        
                        const requestInfo = document.createElement('div');
                        requestInfo.className = 'request-info';
						requestInfo.textContent = (log.method || '') + " " + (log.path || '');
                        logHeader.appendChild(requestInfo);
                        
                        const statusContainer = document.createElement('div');
                        
                        if (log.status_code) {
                            const statusCode = document.createElement('span');
                            statusCode.className = 'status-code ' + getStatusCodeClass(log.status_code);
                            statusCode.textContent = log.status_code;
                            statusContainer.appendChild(statusCode);
                        }
                        
                        if (log.response_time_ms) {
                            const responseTime = document.createElement('span');
                            responseTime.className = 'response-time ' + getResponseTimeClass(log.response_time_ms);
                            responseTime.textContent = ' ' + log.response_time_ms + 'ms';
                            statusContainer.appendChild(responseTime);
                        }
                        
                        logHeader.appendChild(statusContainer);
                        logEntry.appendChild(logHeader);
                        
                        // Create details section
                        const details = document.createElement('div');
                        details.className = 'details';
                        
                        // Basic info
                        const basicInfo = document.createElement('div');
                        basicInfo.className = 'detail-item';
                        
                        const timestamp = document.createElement('div');
                        timestamp.className = 'timestamp';
                        timestamp.textContent = 'Time: ' + formatTimestamp(log.timestamp);
                        basicInfo.appendChild(timestamp);
                        
                        const requestID = document.createElement('div');
                        requestID.textContent = 'Request ID: ' + log.request_id;
                        basicInfo.appendChild(requestID);
                        
                        const topic = document.createElement('div');
                        topic.textContent = 'Topic: ' + log.topic;
                        basicInfo.appendChild(topic);
                        
                        if (log.response_topic) {
                            const responseTopic = document.createElement('div');
                            responseTopic.textContent = 'Response Topic: ' + log.response_topic;
                            basicInfo.appendChild(responseTopic);
                        }
                        
                        if (log.error) {
                            const error = document.createElement('div');
                            error.className = 'error';
                            error.textContent = 'Error: ' + log.error;
                            basicInfo.appendChild(error);
                        }
                        
                        details.appendChild(basicInfo);
                        
                        // Request body
                        if (log.request_body) {
                            const requestBody = document.createElement('div');
                            requestBody.className = 'detail-item';
                            
                            const requestTitle = document.createElement('div');
                            requestTitle.textContent = 'Request Body:';
                            requestBody.appendChild(requestTitle);
                            
                            const requestPre = document.createElement('pre');
                            requestPre.textContent = formatJSON(log.request_body);
                            requestBody.appendChild(requestPre);
                            
                            details.appendChild(requestBody);
                        }
                        
                        // Response body
                        if (log.response_body) {
                            const responseBody = document.createElement('div');
                            responseBody.className = 'detail-item';
                            
                            const responseTitle = document.createElement('div');
                            responseTitle.textContent = 'Response Body:';
                            responseBody.appendChild(responseTitle);
                            
                            const responsePre = document.createElement('pre');
                            responsePre.textContent = formatJSON(log.response_body);
                            responseBody.appendChild(responsePre);
                            
                            details.appendChild(responseBody);
                        }
                        
                        logEntry.appendChild(details);
                        logsContainer.appendChild(logEntry);
                    });
                })
                .catch(error => {
                    logsContainer.innerHTML = '<div class="error">Error: ' + error.message + '</div>';
                });
        }

        // Set up event listeners
        document.addEventListener('DOMContentLoaded', function() {
            // Initial fetch
            fetchLogs();
            
            // Refresh button
            document.getElementById('refresh-btn').addEventListener('click', fetchLogs);
            
            // Limit change
            document.getElementById('limit').addEventListener('change', fetchLogs);
            
            // Auto refresh
            const autoRefreshCheckbox = document.getElementById('auto-refresh');
            const refreshIntervalSelect = document.getElementById('refresh-interval');
            
            function updateAutoRefresh() {
                clearInterval(autoRefreshInterval);
                
                if (autoRefreshCheckbox.checked) {
                    const interval = parseInt(refreshIntervalSelect.value) * 1000;
                    autoRefreshInterval = setInterval(fetchLogs, interval);
                }
            }
            
            autoRefreshCheckbox.addEventListener('change', updateAutoRefresh);
            refreshIntervalSelect.addEventListener('change', updateAutoRefresh);
        });
    </script>
</body>
</html>
`

func renderLogsTemplate(w http.ResponseWriter) {
	// Set content type
	w.Header().Set("Content-Type", "text/html")

	// Parse and execute template
	tmpl, err := template.New("logs").Parse(logsHTMLTemplate)
	if err != nil {
		log.Error().Err(err).Msg("Error parsing logs template")
		http.Error(w, "Error generating logs page", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, nil); err != nil {
		log.Error().Err(err).Msg("Error executing logs template")
	}
}
