package ui

import (
	"html/template"
	"net/http"
)

// QueryUI serves a simple built-in query interface
type QueryUI struct {
	apiURL string
}

// NewQueryUI creates a new query UI handler
func NewQueryUI(apiURL string) *QueryUI {
	return &QueryUI{
		apiURL: apiURL,
	}
}

// ServeHTTP serves the query UI
func (ui *QueryUI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("ui").Parse(queryUITemplate))

	data := struct {
		APIURL string
	}{
		APIURL: ui.apiURL,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}

const queryUITemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PromSketch-Dropin Query UI</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: #f5f5f5;
            padding: 20px;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            padding: 30px;
        }

        h1 {
            color: #333;
            margin-bottom: 10px;
        }

        .subtitle {
            color: #666;
            margin-bottom: 30px;
        }

        .query-section {
            margin-bottom: 30px;
        }

        label {
            display: block;
            margin-bottom: 8px;
            color: #333;
            font-weight: 500;
        }

        .query-input {
            width: 100%;
            padding: 12px;
            font-size: 14px;
            font-family: 'Courier New', monospace;
            border: 1px solid #ddd;
            border-radius: 4px;
            margin-bottom: 10px;
        }

        .query-params {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-bottom: 15px;
        }

        .param-group {
            display: flex;
            flex-direction: column;
        }

        .param-group input {
            padding: 8px;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 14px;
        }

        .button-group {
            display: flex;
            gap: 10px;
        }

        button {
            padding: 10px 20px;
            border: none;
            border-radius: 4px;
            font-size: 14px;
            font-weight: 500;
            cursor: pointer;
            transition: background 0.2s;
        }

        .btn-primary {
            background: #007bff;
            color: white;
        }

        .btn-primary:hover {
            background: #0056b3;
        }

        .btn-secondary {
            background: #6c757d;
            color: white;
        }

        .btn-secondary:hover {
            background: #545b62;
        }

        .results-section {
            margin-top: 30px;
        }

        .results-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 15px;
        }

        .results-header h2 {
            color: #333;
        }

        .result-meta {
            font-size: 12px;
            color: #666;
        }

        .result-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 15px;
        }

        .result-table th,
        .result-table td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }

        .result-table th {
            background: #f8f9fa;
            font-weight: 600;
            color: #333;
        }

        .result-table tr:hover {
            background: #f8f9fa;
        }

        .label-badge {
            display: inline-block;
            padding: 2px 8px;
            margin: 2px;
            background: #e7f3ff;
            border-radius: 3px;
            font-size: 12px;
            font-family: 'Courier New', monospace;
        }

        .error {
            background: #f8d7da;
            color: #721c24;
            padding: 12px;
            border-radius: 4px;
            margin-top: 15px;
        }

        .loading {
            text-align: center;
            padding: 40px;
            color: #666;
        }

        .examples {
            background: #f8f9fa;
            padding: 15px;
            border-radius: 4px;
            margin-bottom: 20px;
        }

        .examples h3 {
            margin-bottom: 10px;
            color: #333;
        }

        .example-query {
            font-family: 'Courier New', monospace;
            font-size: 13px;
            color: #0066cc;
            cursor: pointer;
            padding: 5px;
            display: block;
        }

        .example-query:hover {
            background: #e7f3ff;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔍 PromSketch-Dropin Query UI</h1>
        <p class="subtitle">Execute PromQL queries and explore sketch data</p>

        <div class="examples">
            <h3>Example Queries</h3>
            <span class="example-query" onclick="setQuery('avg_over_time(http_requests_total[5m])')">
                avg_over_time(http_requests_total[5m])
            </span>
            <span class="example-query" onclick="setQuery('sum_over_time(http_requests_total{job=&quot;api&quot;}[5m])')">
                sum_over_time(http_requests_total{job="api"}[5m])
            </span>
            <span class="example-query" onclick="setQuery('quantile_over_time(0.95, http_duration_seconds[5m])')">
                quantile_over_time(0.95, http_duration_seconds[5m])
            </span>
        </div>

        <div class="query-section">
            <label for="query">Query</label>
            <input type="text" id="query" class="query-input" placeholder="Enter PromQL query..."
                   value="avg_over_time(http_requests_total[5m])">

            <div class="query-params">
                <div class="param-group">
                    <label for="queryType">Query Type</label>
                    <select id="queryType" onchange="toggleParams()">
                        <option value="instant">Instant Query</option>
                        <option value="range">Range Query</option>
                    </select>
                </div>

                <div class="param-group" id="timeParam">
                    <label for="time">Time (optional)</label>
                    <input type="text" id="time" placeholder="Unix timestamp or leave empty for now">
                </div>

                <div class="param-group range-param" style="display:none;">
                    <label for="start">Start Time</label>
                    <input type="datetime-local" id="start">
                </div>

                <div class="param-group range-param" style="display:none;">
                    <label for="end">End Time</label>
                    <input type="datetime-local" id="end">
                </div>

                <div class="param-group range-param" style="display:none;">
                    <label for="step">Step</label>
                    <input type="text" id="step" value="60s" placeholder="e.g., 15s, 1m, 5m">
                </div>
            </div>

            <div class="button-group">
                <button class="btn-primary" onclick="executeQuery()">Execute Query</button>
                <button class="btn-secondary" onclick="clearResults()">Clear</button>
            </div>
        </div>

        <div class="results-section" id="results" style="display:none;">
            <div class="results-header">
                <h2>Results</h2>
                <div class="result-meta" id="resultMeta"></div>
            </div>
            <div id="resultContent"></div>
        </div>
    </div>

    <script>
        const apiURL = '{{.APIURL}}';

        function setQuery(query) {
            document.getElementById('query').value = query;
        }

        function toggleParams() {
            const queryType = document.getElementById('queryType').value;
            const rangeParams = document.querySelectorAll('.range-param');
            const timeParam = document.getElementById('timeParam');

            if (queryType === 'range') {
                rangeParams.forEach(p => p.style.display = 'block');
                timeParam.style.display = 'none';

                // Set default times
                const now = new Date();
                const oneHourAgo = new Date(now.getTime() - 60 * 60 * 1000);
                document.getElementById('start').value = formatDateTimeLocal(oneHourAgo);
                document.getElementById('end').value = formatDateTimeLocal(now);
            } else {
                rangeParams.forEach(p => p.style.display = 'none');
                timeParam.style.display = 'block';
            }
        }

        function formatDateTimeLocal(date) {
            const year = date.getFullYear();
            const month = String(date.getMonth() + 1).padStart(2, '0');
            const day = String(date.getDate()).padStart(2, '0');
            const hours = String(date.getHours()).padStart(2, '0');
            const minutes = String(date.getMinutes()).padStart(2, '0');
            return year + '-' + month + '-' + day + 'T' + hours + ':' + minutes;
        }

        async function executeQuery() {
            const query = document.getElementById('query').value;
            const queryType = document.getElementById('queryType').value;

            if (!query) {
                alert('Please enter a query');
                return;
            }

            showLoading();

            try {
                let url, params;

                if (queryType === 'instant') {
                    url = apiURL + '/api/v1/query';
                    params = new URLSearchParams({ query });

                    const time = document.getElementById('time').value;
                    if (time) {
                        params.append('time', time);
                    }
                } else {
                    url = apiURL + '/api/v1/query_range';
                    const start = new Date(document.getElementById('start').value).getTime() / 1000;
                    const end = new Date(document.getElementById('end').value).getTime() / 1000;
                    const step = document.getElementById('step').value;

                    params = new URLSearchParams({
                        query,
                        start: start.toString(),
                        end: end.toString(),
                        step
                    });
                }

                const response = await fetch(url + '?' + params.toString());
                const data = await response.json();

                if (data.status === 'success') {
                    displayResults(data.data, queryType);
                } else {
                    displayError(data.error || 'Query failed');
                }
            } catch (error) {
                displayError('Error: ' + error.message);
            }
        }

        function showLoading() {
            const results = document.getElementById('results');
            const content = document.getElementById('resultContent');
            results.style.display = 'block';
            content.innerHTML = '<div class="loading">Loading...</div>';
        }

        function displayResults(data, queryType) {
            const results = document.getElementById('results');
            const content = document.getElementById('resultContent');
            const meta = document.getElementById('resultMeta');

            results.style.display = 'block';

            if (!data.result || data.result.length === 0) {
                content.innerHTML = '<p>No results found</p>';
                meta.textContent = '';
                return;
            }

            meta.textContent = data.result.length + ' series returned';

            let html = '<table class="result-table"><thead><tr><th>Labels</th>';

            if (queryType === 'instant') {
                html += '<th>Timestamp</th><th>Value</th>';
            } else {
                html += '<th>Values</th>';
            }

            html += '</tr></thead><tbody>';

            data.result.forEach(series => {
                html += '<tr><td>';

                // Display labels
                if (series.metric && Object.keys(series.metric).length > 0) {
                    for (const [key, value] of Object.entries(series.metric)) {
                        html += '<span class="label-badge">' + key + '="' + value + '"</span>';
                    }
                } else {
                    html += '<span class="label-badge">no labels</span>';
                }

                html += '</td>';

                if (queryType === 'instant' && series.value) {
                    html += '<td>' + new Date(series.value[0] * 1000).toISOString() + '</td>';
                    html += '<td>' + series.value[1] + '</td>';
                } else if (series.values) {
                    html += '<td>' + series.values.length + ' data points</td>';
                }

                html += '</tr>';
            });

            html += '</tbody></table>';
            content.innerHTML = html;
        }

        function displayError(message) {
            const results = document.getElementById('results');
            const content = document.getElementById('resultContent');
            results.style.display = 'block';
            content.innerHTML = '<div class="error">' + message + '</div>';
        }

        function clearResults() {
            document.getElementById('results').style.display = 'none';
            document.getElementById('query').value = '';
        }
    </script>
</body>
</html>
`
