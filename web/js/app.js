// UBAAS Frontend Application
// Handles navigation, API calls, and DOM updates

const API_BASE = '';

// Utility: Make API request
async function apiRequest(endpoint, options = {}) {
    const url = API_BASE + endpoint;
    const headers = {
        'Content-Type': 'application/json',
        ...options.headers,
    };

    try {
        const response = await fetch(url, {
            ...options,
            headers,
        });

        const data = await response.json();
        return { ok: response.ok, status: response.status, data };
    } catch (error) {
        return { ok: false, status: 0, data: { message: error.message } };
    }
}

// Navigation
document.querySelectorAll('.nav-link').forEach(link => {
    link.addEventListener('click', (e) => {
        e.preventDefault();
        const view = link.dataset.view;

        // Update active nav
        document.querySelectorAll('.nav-link').forEach(l => l.classList.remove('active'));
        link.classList.add('active');

        // Show view
        document.querySelectorAll('.view').forEach(v => v.classList.remove('active'));
        const viewEl = document.getElementById('view-' + view);
        if (viewEl) {
            viewEl.classList.add('active');
        }

        // Load view data
        loadViewData(view);
    });
});

// View data loading
let currentPage = 1;
const PAGE_SIZE = 20;

function loadViewData(view) {
    switch (view) {
        case 'dashboard':
            loadDashboard();
            break;
        case 'events':
            loadEvents();
            break;
        case 'sessions':
            loadSessions();
            break;
        case 'paths':
            loadPaths();
            break;
        case 'conversions':
            loadConversions();
            break;
        case 'analytics':
            loadAnalytics();
            break;
    }
}

// Dashboard
async function loadDashboard() {
    const result = await apiRequest('/api/stats/overall');
    if (result.ok && result.data.data) {
        const stats = result.data.data;
        document.getElementById('stat-active-sessions').textContent = stats.active_sessions ?? '--';
        document.getElementById('stat-total-events').textContent = stats.total_events ?? '--';
        document.getElementById('stat-today-events').textContent = stats.today_events ?? '--';
        document.getElementById('stat-unique-users').textContent = stats.total_users ?? '--';
    }

    const breakdown = await apiRequest('/api/stats/events');
    if (breakdown.ok && breakdown.data.data) {
        const bd = breakdown.data.data;
        const container = document.getElementById('event-breakdown');
        container.innerHTML = Object.entries(bd)
            .map(([type, count]) => `<div class="breakdown-item">${type}: ${count}</div>`)
            .join('');
    }
}

// Events
async function loadEvents() {
    const userId = document.getElementById('filter-user-id').value;
    const eventType = document.getElementById('filter-event-type').value;

    let url = `/api/events?page=${currentPage}&page_size=${PAGE_SIZE}`;
    if (userId) url += `&user_id=${encodeURIComponent(userId)}`;
    if (eventType) url += `&type=${encodeURIComponent(eventType)}`;

    const result = await apiRequest(url);
    if (result.ok && result.data.data) {
        const events = result.data.data;
        const body = document.getElementById('events-body');
        if (events.length === 0) {
            body.innerHTML = '<tr><td colspan="6">No events found</td></tr>';
        } else {
            body.innerHTML = events.map(e => `
                <tr>
                    <td>${e.id?.substring(0, 8) || ''}...</td>
                    <td>${e.user_id || ''}</td>
                    <td>${e.type || ''}</td>
                    <td>${e.page_url || ''}</td>
                    <td>${e.device_type || ''}</td>
                    <td>${e.timestamp ? new Date(e.timestamp).toLocaleString() : ''}</td>
                </tr>
            `).join('');
        }

        const total = result.data.pagination?.total ?? 0;
        document.getElementById('events-page-info').textContent = `Page ${currentPage} (Total: ${total})`;
    }
}

// Sessions
async function loadSessions() {
    const state = document.getElementById('filter-session-state').value;
    let url = `/api/sessions?page=1&page_size=50`;
    if (state) url += `&state=${encodeURIComponent(state)}`;

    const result = await apiRequest(url);
    if (result.ok && result.data.data) {
        const sessions = result.data.data;
        const body = document.getElementById('sessions-body');
        if (sessions.length === 0) {
            body.innerHTML = '<tr><td colspan="7">No sessions found</td></tr>';
        } else {
            body.innerHTML = sessions.map(s => `
                <tr>
                    <td>${s.id?.substring(0, 8) || ''}...</td>
                    <td>${s.user_id || ''}</td>
                    <td>${s.state || ''}</td>
                    <td>${s.device_type || ''}</td>
                    <td>${s.event_count || 0}</td>
                    <td>${s.start_time ? new Date(s.start_time).toLocaleString() : ''}</td>
                    <td>${s.end_time ? new Date(s.end_time).toLocaleString() : ''}</td>
                </tr>
            `).join('');
        }
    }
}

// Paths
async function loadPaths() {
    // Load hot paths
    const hotResult = await apiRequest('/api/paths/hot?limit=10');
    if (hotResult.ok && hotResult.data.data) {
        const container = document.getElementById('hot-paths');
        const paths = hotResult.data.data.hot_paths || [];
        if (paths.length === 0) {
            container.innerHTML = '<p>No paths data yet. Send some events!</p>';
        } else {
            container.innerHTML = paths.map((p, i) => `
                <div class="hot-path-item">
                    <span class="path-url">${i + 1}. ${p.path || 'N/A'}</span>
                    <span class="path-count">${p.visit_count} visits</span>
                </div>
            `).join('');
        }
    }

    // Load popular pages
    const popResult = await apiRequest('/api/paths/pages/popular?limit=10');
    if (popResult.ok && popResult.data.data) {
        const container = document.getElementById('popular-pages');
        const pages = popResult.data.data.popular_pages || [];
        if (pages.length === 0) {
            container.innerHTML = '<p>No page data yet.</p>';
        } else {
            container.innerHTML = pages.map(p => `
                <div class="popular-page-item">
                    <span class="path-url">${p.page_url || ''}</span>
                    <span class="path-count">${p.view_count} views</span>
                </div>
            `).join('');
        }
    }
}

// Conversions
async function loadConversions() {
    // Load goals
    const result = await apiRequest('/api/conversions/goals');
    if (result.ok && result.data.data) {
        const goals = result.data.data;
        const container = document.getElementById('goals-list');
        if (goals.length === 0) {
            container.innerHTML = '<p>No conversion goals. Create one above.</p>';
        } else {
            container.innerHTML = goals.map(g => `
                <div class="goal-item">
                    <h4>${g.name}</h4>
                    <p>${g.start_page} → ${g.end_page}</p>
                    <p>ID: ${g.id}</p>
                </div>
            `).join('');
        }
    }
}

// Analytics
async function loadAnalytics() {
    // Device breakdown
    const deviceResult = await apiRequest('/api/stats/devices');
    if (deviceResult.ok && deviceResult.data.data) {
        const data = deviceResult.data.data;
        const container = document.getElementById('device-breakdown');
        container.innerHTML = Object.entries(data)
            .map(([device, events]) => {
                const total = Object.values(events).reduce((a, b) => a + b, 0);
                return `
                    <div class="dimension-item">
                        <span class="dim-value">${device}</span>
                        <span>${total} events</span>
                    </div>
                `;
            }).join('');
    }

    // Hourly distribution
    const hourlyResult = await apiRequest('/api/stats/hourly');
    if (hourlyResult.ok && hourlyResult.data.data) {
        const hourly = hourlyResult.data.data.hourly_distribution || {};
        const container = document.getElementById('hourly-dist');
        const maxCount = Math.max(...Object.values(hourly), 1);
        container.innerHTML = Object.entries(hourly)
            .sort((a, b) => parseInt(a[0]) - parseInt(b[0]))
            .map(([hour, count]) => `
                <div>
                    <div class="dimension-item">
                        <span class="dim-value">${hour}:00</span>
                        <span>${count}</span>
                    </div>
                    <div class="dim-bar" style="width: ${(count / maxCount) * 100}%"></div>
                </div>
            `).join('');
    }

    // Country breakdown
    const countryResult = await apiRequest('/api/stats/countries');
    if (countryResult.ok && countryResult.data.data) {
        const countries = countryResult.data.data || [];
        const container = document.getElementById('country-breakdown');
        if (countries.length === 0) {
            container.innerHTML = '<p>No data.</p>';
        } else {
            container.innerHTML = countries.slice(0, 10).map(c => `
                <div class="dimension-item">
                    <span class="dim-value">${c.value}</span>
                    <span>${c.count} (${c.percent.toFixed(1)}%)</span>
                </div>
            `).join('');
        }
    }
}

// Event search buttons
document.getElementById('btn-search-events')?.addEventListener('click', () => {
    currentPage = 1;
    loadEvents();
});

document.getElementById('btn-search-sessions')?.addEventListener('click', loadSessions);

document.getElementById('btn-prev-events')?.addEventListener('click', () => {
    if (currentPage > 1) {
        currentPage--;
        loadEvents();
    }
});

document.getElementById('btn-next-events')?.addEventListener('click', () => {
    currentPage++;
    loadEvents();
});

// Conversion calculate
document.getElementById('btn-calc-conversion')?.addEventListener('click', async () => {
    const startPage = document.getElementById('conv-start-page').value;
    const endPage = document.getElementById('conv-end-page').value;

    if (!startPage || !endPage) {
        alert('Please provide start and end pages');
        return;
    }

    const now = new Date();
    const oneDayAgo = new Date(now.getTime() - 24 * 60 * 60 * 1000);

    const url = `/api/conversions/rate?start_page=${encodeURIComponent(startPage)}&end_page=${encodeURIComponent(endPage)}&start_date=${oneDayAgo.toISOString()}&end_date=${now.toISOString()}`;
    const result = await apiRequest(url);

    const container = document.getElementById('conversion-result');
    if (result.ok && result.data.data) {
        const data = result.data.data;
        container.innerHTML = `
            <div style="text-align: center;">
                <div class="conversion-rate">${data.conversion_rate?.toFixed(2) || '0'}%</div>
                <p>Visitors: ${data.total_visitors || 0} | Converted: ${data.converted_users || 0}</p>
                <p>Start: ${data.start_page} → End: ${data.end_page}</p>
            </div>
        `;
    } else {
        container.innerHTML = '<p>Failed to calculate conversion rate</p>';
    }
});

// Create goal
document.getElementById('btn-create-goal')?.addEventListener('click', async () => {
    const name = document.getElementById('goal-name').value;
    const startPage = document.getElementById('goal-start').value;
    const endPage = document.getElementById('goal-end').value;

    if (!name || !startPage || !endPage) {
        alert('Please provide name, start page, and end page');
        return;
    }

    const result = await apiRequest('/api/conversions/goals', {
        method: 'POST',
        body: JSON.stringify({ name, start_page: startPage, end_page: endPage }),
    });

    if (result.ok) {
        alert('Goal created successfully!');
        loadConversions();
    } else {
        alert('Failed to create goal: ' + (result.data.message || ''));
    }
});

// Ingest events
document.getElementById('btn-send-event')?.addEventListener('click', async () => {
    const event = {
        user_id: document.getElementById('ingest-user-id').value,
        type: document.getElementById('ingest-event-type').value,
        page_url: document.getElementById('ingest-page-url').value,
        device_type: document.getElementById('ingest-device-type').value,
        duration_ms: parseInt(document.getElementById('ingest-duration').value) || 0,
        timestamp: new Date().toISOString(),
    };

    const result = await apiRequest('/api/events', {
        method: 'POST',
        body: JSON.stringify(event),
    });

    showIngestResult(result);
});

document.getElementById('btn-send-batch')?.addEventListener('click', async () => {
    const events = [];
    const pages = ['/home', '/products', '/cart', '/checkout', '/success', '/profile', '/settings'];
    const types = ['page_view', 'click', 'duration'];

    for (let i = 0; i < 50; i++) {
        events.push({
            user_id: `batch-user-${Math.floor(Math.random() * 10)}`,
            type: types[Math.floor(Math.random() * types.length)],
            page_url: pages[Math.floor(Math.random() * pages.length)],
            device_type: Math.random() > 0.5 ? 'desktop' : 'mobile',
            duration_ms: Math.floor(Math.random() * 10000),
            timestamp: new Date().toISOString(),
        });
    }

    const result = await apiRequest('/api/events/batch', {
        method: 'POST',
        body: JSON.stringify(events),
    });

    showIngestResult(result);
});

function showIngestResult(result) {
    const container = document.getElementById('ingest-result');
    if (result.ok) {
        container.className = 'ingest-result show';
        container.innerHTML = `✓ Success! Event created with ID: ${result.data.data?.id || 'batch'}`;
    } else {
        container.className = 'ingest-result show';
        container.style.background = '#f8d7da';
        container.style.color = '#721c24';
        container.innerHTML = `✗ Error: ${result.data.message || 'Unknown error'}`;
    }

    setTimeout(() => {
        container.style.background = '';
        container.style.color = '';
        container.className = 'ingest-result';
    }, 3000);
}

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    loadDashboard();
});

// Auto-refresh dashboard every 30 seconds
setInterval(() => {
    const activeView = document.querySelector('.view.active');
    if (activeView && activeView.id === 'view-dashboard') {
        loadDashboard();
    }
}, 30000);
