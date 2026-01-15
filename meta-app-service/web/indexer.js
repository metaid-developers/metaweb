// API base URL
const API_BASE = window.location.origin + window.location.pathname.replace(/indexer\.html$/, '').replace(/\/$/, '');

// Metafs domain - can be configured via environment or default
// Will be fetched from API or use default
let METAFS_DOMAIN = window.METAFS_DOMAIN || 'http://localhost:7281';

// Load config from API on page load
async function loadConfig() {
    try {
        const response = await fetch(`${API_BASE}/api/v1/config`);
        const data = await response.json();
        if (data.code === 0 && data.data && data.data.metafs_domain) {
            METAFS_DOMAIN = data.data.metafs_domain;
            console.log('✅ Loaded METAFS_DOMAIN from API:', METAFS_DOMAIN);
        }
    } catch (error) {
        console.warn('⚠️ Failed to load config from API, using default:', error);
    }
}

// Helper function to get metafile URL
function getMetafileUrl(metafilePinId) {
    if (!metafilePinId) return '';
    if (!metafilePinId.startsWith('metafile://')) return metafilePinId;
    
    const pinId = metafilePinId.replace('metafile://', '');
    return `${METAFS_DOMAIN}/api/v1/files/accelerate/content/${pinId}?process=thumbnail`;
}

// Global state
let walletConnected = false;
let currentAddress = null;
let currentMetaID = null;
let appsCursor = 0;
let hasMoreApps = true;
let isLoadingApps = false;
let currentView = 'all'; // 'all' or 'my'

// DOM elements
const connectBtn = document.getElementById('connectBtn');
const disconnectBtn = document.getElementById('disconnectBtn');
const walletStatus = document.getElementById('walletStatus');
const walletAddress = document.getElementById('walletAddress');
const addressText = document.getElementById('addressText');
const metaidText = document.getElementById('metaidText');
const walletAlert = document.getElementById('walletAlert');
const appListSection = document.getElementById('appListSection');
const appListContainer = document.getElementById('appListContainer');
const loadMoreBtn = document.getElementById('loadMoreBtn');
const refreshStatusBtn = document.getElementById('refreshStatusBtn');
const refreshAppsBtn = document.getElementById('refreshAppsBtn');
const appListTitle = document.getElementById('appListTitle');
const viewAllBtn = document.getElementById('viewAllBtn');
const viewMyBtn = document.getElementById('viewMyBtn');

// Status elements
const currentBlockEl = document.getElementById('currentBlock');
const latestBlockEl = document.getElementById('latestBlock');
const totalAppsEl = document.getElementById('totalApps');
const syncProgressEl = document.getElementById('syncProgress');

// Initialization
window.addEventListener('load', async () => {
    console.log('🚀 MetaApp Indexer page loaded');
    
    // Load config first (including METAFS_DOMAIN)
    await loadConfig();
    
    initWalletCheck();
    initEventListeners();
    loadIndexerStatus();
    loadAllApps();
    
    // Auto refresh status every 30 seconds
    setInterval(loadIndexerStatus, 30000);
});

// Check Metalet wallet
function initWalletCheck() {
    const isMobile = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent);
    const isInApp = window.navigator.standalone || window.matchMedia('(display-mode: standalone)').matches;
    
    const walletObject = detectWallet();
    
    if (walletObject) {
        handleWalletDetected(walletObject);
    } else if (isMobile || isInApp) {
        console.log('Mobile environment, retrying wallet detection...');
        retryWalletDetection(3, 1000);
    }
}

function detectWallet() {
    if (typeof window.metaidwallet !== 'undefined') {
        return { object: window.metaidwallet, type: 'Metalet Wallet' };
    }
    return null;
}

function handleWalletDetected(walletInfo) {
    window.detectedWallet = walletInfo.object;
    window.walletType = walletInfo.type;
    walletAlert.classList.add('hidden');
}

function retryWalletDetection(attempts, delay) {
    if (attempts <= 0) {
        walletAlert.classList.remove('hidden');
        return;
    }
    
    setTimeout(() => {
        const walletObject = detectWallet();
        if (walletObject) {
            handleWalletDetected(walletObject);
        } else {
            retryWalletDetection(attempts - 1, delay);
        }
    }, delay);
}

function getWallet() {
    return window.detectedWallet || window.metaidwallet;
}

// Initialize event listeners
function initEventListeners() {
    if (connectBtn) {
        connectBtn.addEventListener('click', connectWallet);
    }
    
    if (disconnectBtn) {
        disconnectBtn.addEventListener('click', disconnectWallet);
    }
    
    if (refreshStatusBtn) {
        refreshStatusBtn.addEventListener('click', loadIndexerStatus);
    }
    
    if (refreshAppsBtn) {
        refreshAppsBtn.addEventListener('click', refreshAppList);
    }
    
    if (loadMoreBtn) {
        loadMoreBtn.addEventListener('click', loadMoreApps);
    }
}

// Connect wallet
async function connectWallet() {
    console.log('🔵 Connecting wallet...');
    
    const wallet = getWallet();
    if (!wallet) {
        showNotification('Please install Metalet wallet extension first!', 'error');
        return;
    }

    try {
        connectBtn.disabled = true;
        connectBtn.textContent = 'Connecting...';
        
        const account = await wallet.connect();
        const address = account.address || account.mvcAddress || account.btcAddress;
        
        if (account && address) {
            currentAddress = address;
            walletConnected = true;
            
            walletStatus.textContent = 'Connected';
            walletStatus.style.color = '#28a745';
            
            addressText.textContent = currentAddress;
            
            // Calculate MetaID
            currentMetaID = await calculateMetaID(currentAddress);
            metaidText.textContent = currentMetaID;
            
            walletAddress.classList.remove('hidden');
            walletAlert.classList.add('hidden');
            
            connectBtn.classList.add('hidden');
            disconnectBtn.classList.remove('hidden');
            
            showNotification('Wallet connected successfully!', 'success');
            
            // If viewing "My Apps", reload the list
            if (currentView === 'my') {
                loadMyApps();
            }
        }
    } catch (error) {
        console.error('Failed to connect wallet:', error);
        showNotification('Failed to connect wallet: ' + error.message, 'error');
        connectBtn.disabled = false;
        connectBtn.textContent = 'Connect Metalet Wallet';
    }
}

// Disconnect wallet
function disconnectWallet() {
    walletConnected = false;
    currentAddress = null;
    currentMetaID = null;
    
    walletStatus.textContent = 'Not Connected';
    walletStatus.style.color = '#999';
    walletAddress.classList.add('hidden');
    
    connectBtn.classList.remove('hidden');
    connectBtn.textContent = 'Connect Metalet Wallet';
    connectBtn.disabled = false;
    
    disconnectBtn.classList.add('hidden');
    
    // If viewing "My Apps", switch to "All Apps"
    if (currentView === 'my') {
        switchView('all');
    }
    
    showNotification('Wallet disconnected', 'info');
}

// Calculate MetaID (SHA256 of address)
async function calculateMetaID(address) {
    try {
        const encoder = new TextEncoder();
        const data = encoder.encode(address);
        const hashBuffer = await crypto.subtle.digest('SHA-256', data);
        const hashArray = Array.from(new Uint8Array(hashBuffer));
        const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
        return hashHex;
    } catch (error) {
        console.error('Failed to calculate MetaID:', error);
        return '';
    }
}

// Load indexer status
async function loadIndexerStatus() {
    try {
        // Get sync status from API
        const statusResponse = await fetch(`${API_BASE}/api/v1/status`);
        const statusData = await statusResponse.json();
        
        if (statusData.code === 0 && statusData.data) {
            const status = statusData.data;
            
            // Update current sync height
            currentBlockEl.textContent = status.current_sync_height.toLocaleString();
            
            // Update latest block height from node
            if (status.latest_block_height && status.latest_block_height > 0) {
                latestBlockEl.textContent = status.latest_block_height.toLocaleString();
                
                // Calculate sync progress
                if (status.current_sync_height >= status.latest_block_height) {
                    syncProgressEl.textContent = '✅ Synced';
                } else {
                    const progress = ((status.current_sync_height / status.latest_block_height) * 100).toFixed(2);
                    const behind = status.latest_block_height - status.current_sync_height;
                    syncProgressEl.textContent = `⏳ Syncing (${progress}%, ${behind.toLocaleString()} blocks behind)`;
                }
            } else {
                latestBlockEl.textContent = '-';
                syncProgressEl.textContent = '✅ Running';
            }
            
            console.log('✅ Indexer status loaded:', status);
            
            // Get statistics (total apps count)
            const statsResponse = await fetch(`${API_BASE}/api/v1/stats`);
            const statsData = await statsResponse.json();
            
            if (statsData.code === 0 && statsData.data) {
                totalAppsEl.textContent = statsData.data.total_apps.toLocaleString();
                console.log('📊 Total apps:', statsData.data.total_apps);
            } else {
                totalAppsEl.textContent = '-';
            }
        } else {
            throw new Error(statusData.message || 'Failed to load status');
        }
    } catch (error) {
        console.error('Failed to load indexer status:', error);
        currentBlockEl.textContent = 'Error';
        latestBlockEl.textContent = 'Error';
        totalAppsEl.textContent = 'Error';
        syncProgressEl.textContent = 'Error';
    }
}

// Switch view between "All Apps" and "My Apps"
function switchView(view) {
    currentView = view;
    
    if (view === 'all') {
        viewAllBtn.classList.add('active');
        viewMyBtn.classList.remove('active');
        appListTitle.textContent = '📱 All MetaApps';
        loadAllApps();
    } else if (view === 'my') {
        viewMyBtn.classList.add('active');
        viewAllBtn.classList.remove('active');
        appListTitle.textContent = '📱 My MetaApps';
        
        if (walletConnected && currentMetaID) {
            loadMyApps();
        } else {
            showNotification('Please connect wallet to view your apps', 'warning');
            appListContainer.innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon">🔐</div>
                    <p>Please connect your wallet to view your MetaApps</p>
                </div>
            `;
        }
    }
}

// Load all apps
async function loadAllApps() {
    appListContainer.innerHTML = '<div class="loading"><div class="spinner"></div><p style="margin-top: 10px;">Loading apps...</p></div>';
    
    try {
        appsCursor = 0;
        hasMoreApps = true;
        
        const response = await fetch(`${API_BASE}/api/v1/metaapps?cursor=0&size=20`);
        const data = await response.json();
        
        if (data.code === 0) {
            const apps = data.data.apps || [];
            const nextCursor = data.data.next_cursor || 0;
            hasMoreApps = data.data.has_more || false;
            
            if (apps.length === 0) {
                appListContainer.innerHTML = `
                    <div class="empty-state">
                        <div class="empty-state-icon">📭</div>
                        <p>No MetaApps found</p>
                    </div>
                `;
                loadMoreBtn.classList.add('hidden');
            } else {
                appsCursor = nextCursor;
                renderApps(apps, true);
                
                if (hasMoreApps) {
                    loadMoreBtn.classList.remove('hidden');
                } else {
                    loadMoreBtn.classList.add('hidden');
                }
            }
        } else {
            throw new Error(data.message || 'Failed to load apps');
        }
    } catch (error) {
        console.error('Failed to load apps:', error);
        appListContainer.innerHTML = `
            <div class="empty-state">
                <div class="empty-state-icon">❌</div>
                <p>Failed to load apps</p>
                <p style="font-size: 14px; margin-top: 10px;">${error.message}</p>
            </div>
        `;
        showNotification('Failed to load apps: ' + error.message, 'error');
    }
}

// Load my apps
async function loadMyApps() {
    if (!currentMetaID) {
        console.error('MetaID not available');
        return;
    }
    
    appListContainer.innerHTML = '<div class="loading"><div class="spinner"></div><p style="margin-top: 10px;">Loading your apps...</p></div>';
    
    try {
        appsCursor = 0;
        hasMoreApps = true;
        
        const response = await fetch(`${API_BASE}/api/v1/metaapps/creator/${currentMetaID}?cursor=0&size=20`);
        const data = await response.json();
        
        if (data.code === 0) {
            const apps = data.data.apps || [];
            const nextCursor = data.data.next_cursor || 0;
            hasMoreApps = data.data.has_more || false;
            
            if (apps.length === 0) {
                appListContainer.innerHTML = `
                    <div class="empty-state">
                        <div class="empty-state-icon">📭</div>
                        <p>No MetaApps found</p>
                        <p style="font-size: 14px; margin-top: 10px;">You haven't created any MetaApps yet!</p>
                    </div>
                `;
                loadMoreBtn.classList.add('hidden');
            } else {
                appsCursor = nextCursor;
                renderApps(apps, true);
                
                if (hasMoreApps) {
                    loadMoreBtn.classList.remove('hidden');
                } else {
                    loadMoreBtn.classList.add('hidden');
                }
            }
        } else {
            throw new Error(data.message || 'Failed to load apps');
        }
    } catch (error) {
        console.error('Failed to load my apps:', error);
        appListContainer.innerHTML = `
            <div class="empty-state">
                <div class="empty-state-icon">❌</div>
                <p>Failed to load apps</p>
                <p style="font-size: 14px; margin-top: 10px;">${error.message}</p>
            </div>
        `;
        showNotification('Failed to load apps: ' + error.message, 'error');
    }
}

// Load more apps
async function loadMoreApps() {
    if (isLoadingApps || !hasMoreApps) {
        return;
    }
    
    isLoadingApps = true;
    loadMoreBtn.disabled = true;
    loadMoreBtn.textContent = 'Loading...';
    
    try {
        let url;
        if (currentView === 'all') {
            url = `${API_BASE}/api/v1/metaapps?cursor=${appsCursor}&size=20`;
        } else {
            if (!currentMetaID) {
                throw new Error('MetaID not available');
            }
            url = `${API_BASE}/api/v1/metaapps/creator/${currentMetaID}?cursor=${appsCursor}&size=20`;
        }
        
        const response = await fetch(url);
        const data = await response.json();
        
        if (data.code === 0) {
            const apps = data.data.apps || [];
            const nextCursor = data.data.next_cursor || 0;
            hasMoreApps = data.data.has_more || false;
            
            if (apps.length > 0) {
                appsCursor = nextCursor;
                renderApps(apps, false);
            }
            
            if (!hasMoreApps) {
                loadMoreBtn.classList.add('hidden');
            }
        } else {
            throw new Error(data.message || 'Failed to load more apps');
        }
    } catch (error) {
        console.error('Failed to load more apps:', error);
        showNotification('Failed to load more apps: ' + error.message, 'error');
    } finally {
        isLoadingApps = false;
        loadMoreBtn.disabled = false;
        loadMoreBtn.textContent = 'Load More';
    }
}

// Refresh app list
function refreshAppList() {
    if (currentView === 'all') {
        loadAllApps();
    } else if (currentView === 'my' && currentMetaID) {
        loadMyApps();
    }
}

// Render apps
function renderApps(apps, clearFirst) {
    if (clearFirst) {
        appListContainer.innerHTML = '';
    }
    
    apps.forEach(app => {
        const appCard = createAppCard(app);
        appListContainer.appendChild(appCard);
    });
}

// Create app card - 按照卡片样式设计
function createAppCard(app) {
    const card = document.createElement('div');
    card.className = 'app-card';
    
    // Get icon and cover image URLs
    const iconUrl = getMetafileUrl(app.icon || '');
    const coverUrl = getMetafileUrl(app.cover_img || '');
    
    // Get deploy status
    let deployStatus = '';
    let deployStatusClass = '';
    let isDeployed = false;
    if (app.deploy_info) {
        const status = app.deploy_info.deploy_status;
        if (status === 'completed') {
            deployStatus = '✅ 已部署';
            deployStatusClass = 'completed';
            isDeployed = true;
        } else if (status === 'processing') {
            deployStatus = '⏳ 部署中';
            deployStatusClass = 'processing';
        } else if (status === 'failed') {
            deployStatus = '❌ 部署失败';
            deployStatusClass = 'failed';
        } else {
            deployStatus = '⏸️ 待部署';
            deployStatusClass = 'pending';
        }
    } else {
        deployStatus = '⏸️ 待部署';
        deployStatusClass = 'pending';
    }
    
    // Format date
    const releaseDate = new Date(app.timestamp).toLocaleDateString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit'
    });
    
    // Format PINID (show first 4 and last 4 characters)
    const pinIdDisplay = app.pin_id.length > 12 
        ? `${app.pin_id.substring(0, 4)}...${app.pin_id.substring(app.pin_id.length - 4)}`
        : app.pin_id;
    
    // Format FirstPinID (show first 4 and last 4 characters)
    const firstPinId = app.first_pin_id || app.pin_id;
    const firstPinIdDisplay = firstPinId.length > 12 
        ? `${firstPinId.substring(0, 4)}...${firstPinId.substring(firstPinId.length - 4)}`
        : firstPinId;
    
    // Format CreatorMetaId (show first 6 characters)
    const creatorMetaIdDisplay = app.creator_meta_id 
        ? (app.creator_meta_id.length > 6 ? `${app.creator_meta_id.substring(0, 6)}` : app.creator_meta_id)
        : 'N/A';
    
    // App name
    const appName = app.app_name || app.title || 'Unnamed App';
    
    // Version
    const version = app.version || 'v1.0.0';
    
    // 部署消息（所有状态都可以显示）
    const deployMessage = app.deploy_info && app.deploy_info.deploy_message 
        ? app.deploy_info.deploy_message 
        : '';
    const hasDeployMessage = deployMessage && deployMessage.trim() !== '';
    
    card.innerHTML = `
        <!-- Cover Image -->
        ${coverUrl ? `
        <div class="app-cover-container">
            <img src="${coverUrl}" alt="Cover" class="app-cover" onerror="this.style.display='none'; this.parentElement.style.display='none';">
        </div>
        ` : ''}
        
        <!-- App Header with Icon -->
        <div class="app-header">
            ${iconUrl ? `
            <img src="${iconUrl}" alt="Icon" class="app-icon" onerror="this.style.display='none'">
            ` : '<div class="app-icon-placeholder">📱</div>'}
            <div class="app-header-content">
                <div class="app-title">${escapeHtml(appName)}</div>
                <div class="app-version">
                    <span class="version-clickable" onclick="showVersionHistory('${firstPinId}')" title="点击查看历史版本">
                        ${version} ▼
                    </span>
                </div>
            </div>
        </div>
        
        <!-- App Body -->
        <div class="app-body">
            <!-- FirstPinID -->
            <div class="app-pinid clickable" onclick="copyToClipboard('${firstPinId}')" title="点击复制 FirstPinID: ${firstPinId}">
                <strong>FirstPinID:</strong> ${firstPinIdDisplay} 📋
            </div>
            
            <!-- Current PINID (始终显示) -->
            <div class="app-pinid clickable" onclick="copyToClipboard('${app.pin_id}')" title="点击复制当前 PinID: ${app.pin_id}">
                <strong>PinID:</strong> ${pinIdDisplay} 📋
            </div>
            
            <!-- Release Date -->
            <div class="app-date">
                发布于: ${releaseDate}
            </div>
            
            <!-- Intro -->
            ${app.intro ? `
            <div class="app-intro">
                ${escapeHtml(app.intro)}
            </div>
            ` : ''}
            
            <!-- Developer Info -->
            <div class="app-developer">
                <strong>开发者:</strong> 
                ${app.creator_meta_id ? `
                <span class="metaid-clickable" onclick="copyToClipboard('${app.creator_meta_id}')" title="点击复制 MetaID: ${app.creator_meta_id}">
                    ${escapeHtml(creatorMetaIdDisplay)} 📋
                </span>
                ` : '<span>N/A</span>'}
                ${hasDeployMessage ? `
                <span class="deploy-status ${deployStatusClass} deploy-status-clickable" onclick="showDeployMessage('${escapeHtml(deployMessage)}', '${deployStatusClass}')" title="点击查看部署信息">
                    ${deployStatus}
                </span>
                ` : `
                <span class="deploy-status ${deployStatusClass}">${deployStatus}</span>
                `}
            </div>
        </div>
        
        <!-- App Footer with Actions -->
        <div class="app-footer">
            <button class="btn-action btn-download" onclick="downloadApp('${app.first_pin_id}')">
                下载
            </button>
            ${isDeployed ? `
            <button class="btn-action btn-run" onclick="runApp('${app.first_pin_id}')">
                运行
            </button>
            ` : deployStatusClass === 'failed' ? `
            <button class="btn-action btn-run" onclick="redeployApp('${app.pin_id}')" title="重新部署应用">
                重新部署
            </button>
            ` : `
            <button class="btn-action btn-run" disabled style="opacity: 0.5; cursor: not-allowed;" title="应用尚未部署完成">
                运行
            </button>
            `}
        </div>
    `;
    
    return card;
}

// Helper function to escape HTML
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Download app handler
// 参数可以是 pinId 或 firstPinId
async function downloadApp(pinIdOrFirstPinId) {
    try {
        showNotification('正在准备下载...', 'info');
        
        let firstPinId = pinIdOrFirstPinId;
        
        // 如果传入的是 pinId（而不是 firstPinId），需要先获取应用信息
        // 尝试直接使用，如果失败再通过 API 获取
        try {
            const response = await fetch(`${API_BASE}/api/v1/metaapps/${pinIdOrFirstPinId}`);
            const data = await response.json();
            
            if (data.code === 0 && data.data) {
                const app = data.data;
                firstPinId = app.first_pin_id || pinIdOrFirstPinId;
                
                // 检查部署状态
                const deployStatus = app.deploy_info ? app.deploy_info.deploy_status : null;
                if (deployStatus !== 'completed') {
                    showNotification('应用尚未部署完成，无法下载', 'warning');
                    return;
                }
            } else {
                // 如果获取失败，假设传入的就是 firstPinId
                firstPinId = pinIdOrFirstPinId;
            }
        } catch (error) {
            // 如果出错，假设传入的就是 firstPinId
            console.warn('Failed to get app info, assuming input is firstPinId:', error);
            firstPinId = pinIdOrFirstPinId;
        }
        
        // 构建下载 URL
        const downloadUrl = `${API_BASE}/api/v1/metaapps/first/${firstPinId}/download`;
        
        // 创建一个隐藏的 a 标签来触发下载
        const link = document.createElement('a');
        link.href = downloadUrl;
        link.download = `${firstPinId}.zip`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        
        showNotification('下载已开始...', 'success');
    } catch (error) {
        console.error('Failed to download app:', error);
        showNotification('下载失败: ' + error.message, 'error');
    }
}

// Run app handler
async function runApp(pinId) {
    try {
        // 检查应用部署状态
        const response = await fetch(`${API_BASE}/api/v1/metaapps/${pinId}`);
        const data = await response.json();
        
        if (data.code === 0 && data.data) {
            const app = data.data;
            const deployStatus = app.deploy_info ? app.deploy_info.deploy_status : null;
            
            if (deployStatus === 'completed') {
                // 部署成功，在新标签页中打开应用
                const appUrl = `${API_BASE}/${pinId}/index.html`;
                window.open(appUrl, '_blank');
                showNotification('正在打开应用...', 'success');
            } else {
                showNotification('应用尚未部署完成，无法运行', 'warning');
            }
        } else {
            showNotification('无法获取应用信息', 'error');
        }
    } catch (error) {
        console.error('Failed to run app:', error);
        showNotification('运行应用失败: ' + error.message, 'error');
    }
}

// Redeploy app handler
async function redeployApp(pinId) {
    if (!confirm('确定要重新部署此应用吗？')) {
        return;
    }
    
    try {
        showNotification('正在重新部署应用...', 'info');
        
        const response = await fetch(`${API_BASE}/api/v1/metaapps/${pinId}/redeploy`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        });
        
        const data = await response.json();
        
        if (data.code === 0) {
            showNotification('应用已加入部署队列，5秒后自动刷新...', 'success');
            // 5秒后刷新应用列表
            setTimeout(() => {
                if (currentView === 'all') {
                    loadAllApps();
                } else if (currentView === 'my' && currentMetaID) {
                    loadMyApps();
                }
            }, 5000);
        } else {
            showNotification(data.message || '重新部署失败', 'error');
        }
    } catch (error) {
        console.error('Failed to redeploy app:', error);
        showNotification('重新部署失败: ' + error.message, 'error');
    }
}

// View app detail
async function viewAppDetail(pinId) {
    try {
        const response = await fetch(`${API_BASE}/api/v1/metaapps/${pinId}`);
        const data = await response.json();
        
        if (data.code === 0 && data.data) {
            const app = data.data;
            // Show app details in a modal or alert
            const detailText = `
Title: ${app.title || app.app_name || 'N/A'}
Version: ${app.version || 'N/A'}
Runtime: ${app.runtime || 'N/A'}
Intro: ${app.intro || 'N/A'}
Deploy Status: ${app.deploy_info ? app.deploy_info.deploy_status : 'N/A'}
            `.trim();
            alert(detailText);
        } else {
            showNotification('Failed to load app details', 'error');
        }
    } catch (error) {
        console.error('Failed to load app details:', error);
        showNotification('Failed to load app details: ' + error.message, 'error');
    }
}

// Copy to clipboard
function copyToClipboard(text) {
    // 检查 navigator.clipboard 是否可用
    if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(() => {
            showNotification('已复制到剪贴板: ' + text.substring(0, 20) + '...', 'success');
        }).catch(err => {
            console.error('Failed to copy:', err);
            // 降级方案：使用传统方法
            fallbackCopyToClipboard(text);
        });
    } else {
        // 直接使用降级方案
        fallbackCopyToClipboard(text);
    }
}

// 降级复制方案
function fallbackCopyToClipboard(text) {
    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed';
    textArea.style.top = '0';
    textArea.style.left = '0';
    textArea.style.opacity = '0';
    textArea.style.pointerEvents = 'none';
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    try {
        const successful = document.execCommand('copy');
        if (successful) {
            showNotification('已复制到剪贴板: ' + text.substring(0, 20) + '...', 'success');
        } else {
            showNotification('复制失败，请手动复制', 'error');
        }
    } catch (err) {
        console.error('Fallback copy failed:', err);
        showNotification('复制失败，请手动复制', 'error');
    }
    document.body.removeChild(textArea);
}

// Show deploy message (for all statuses)
function showDeployMessage(message, statusClass) {
    if (!message || message.trim() === '') {
        showNotification('暂无部署信息', 'info');
        return;
    }
    
    // 根据状态设置标题和图标
    let title = '部署信息';
    let icon = 'ℹ️';
    if (statusClass === 'failed') {
        title = '❌ 部署失败原因';
        icon = '❌';
    } else if (statusClass === 'processing') {
        title = '⏳ 部署中';
        icon = '⏳';
    } else if (statusClass === 'completed') {
        title = '✅ 部署完成';
        icon = '✅';
    } else {
        title = '⏸️ 部署状态';
        icon = '⏸️';
    }
    
    // 创建部署信息模态框
    const modal = document.createElement('div');
    modal.className = 'modal-overlay show';
    modal.onclick = function(e) {
        if (e.target === modal) {
            modal.remove();
        }
    };
    
    modal.innerHTML = `
        <div class="modal" onclick="event.stopPropagation()">
            <div class="modal-header">
                <div class="modal-title">${icon} ${title}</div>
                <button class="modal-close" onclick="this.closest('.modal-overlay').remove()">×</button>
            </div>
            <div style="padding: 20px;">
                <div style="background: #f8f9fa; padding: 15px; border-radius: 8px; font-family: monospace; font-size: 13px; color: #333; white-space: pre-wrap; word-break: break-word; max-height: 400px; overflow-y: auto;">
                    ${escapeHtml(message)}
                </div>
            </div>
        </div>
    `;
    
    document.body.appendChild(modal);
}

// Show notification
function showNotification(message, type = 'info') {
    const notification = document.createElement('div');
    notification.className = `notification notification-${type}`;
    
    let icon = '💡';
    if (type === 'success') icon = '✅';
    if (type === 'error') icon = '❌';
    if (type === 'warning') icon = '⚠️';
    
    notification.innerHTML = `
        <span class="notification-icon">${icon}</span>
        <span class="notification-message">${message}</span>
        <button class="notification-close" onclick="this.parentElement.remove()">×</button>
    `;
    
    document.body.appendChild(notification);
    
    setTimeout(() => {
        notification.classList.add('notification-fade-out');
        setTimeout(() => {
            if (notification.parentElement) {
                notification.remove();
            }
        }, 300);
    }, 3000);
}

// Listen for wallet monitoring
let walletCheckInterval = null;

function startWalletMonitoring() {
    const isMobile = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent);
    const isInApp = window.navigator.standalone || window.matchMedia('(display-mode: standalone)').matches;
    
    if (isMobile || isInApp) {
        walletCheckInterval = setInterval(() => {
            if (typeof window.metaidwallet !== 'undefined' && !window.detectedWallet) {
                clearInterval(walletCheckInterval);
                walletCheckInterval = null;
                
                const walletObject = detectWallet();
                if (walletObject) {
                    handleWalletDetected(walletObject);
                }
            }
        }, 500);
        
        setTimeout(() => {
            if (walletCheckInterval) {
                clearInterval(walletCheckInterval);
                walletCheckInterval = null;
            }
        }, 10000);
    }
}

window.addEventListener('load', () => {
    setTimeout(startWalletMonitoring, 1000);
});

// Show version history modal
async function showVersionHistory(firstPinId) {
    if (!firstPinId) {
        showNotification('无法获取版本历史：缺少 first_pin_id', 'error');
        return;
    }

    const modal = document.getElementById('versionHistoryModal');
    const content = document.getElementById('versionHistoryContent');
    
    // Show modal with loading state
    modal.classList.add('show');
    content.innerHTML = '<div class="modal-loading"><div class="spinner"></div><p style="margin-top: 10px;">加载历史版本...</p></div>';

    try {
        const response = await fetch(`${API_BASE}/api/v1/metaapps/first/${firstPinId}/history`);
        const data = await response.json();

        if (data.code === 0 && data.data && data.data.history) {
            const history = data.data.history;
            
            if (history.length === 0) {
                content.innerHTML = '<div class="empty-state"><div class="empty-state-icon">📭</div><p>暂无历史版本</p></div>';
                return;
            }

            // Render version list
            content.innerHTML = history.map((version, index) => {
                const versionDate = new Date(version.timestamp).toLocaleDateString('zh-CN', {
                    year: 'numeric',
                    month: '2-digit',
                    day: '2-digit',
                    hour: '2-digit',
                    minute: '2-digit'
                });
                
                const versionNumber = version.version || 'v1.0.0';
                const pinIdDisplay = version.pin_id.length > 16 
                    ? `${version.pin_id.substring(0, 8)}...${version.pin_id.substring(version.pin_id.length - 8)}`
                    : version.pin_id;
                
                // Check deploy status
                const isDeployed = version.deploy_info && version.deploy_info.deploy_status === 'completed';
                const deployStatus = version.deploy_info 
                    ? (version.deploy_info.deploy_status === 'completed' ? '✅ 部署成功' 
                       : version.deploy_info.deploy_status === 'processing' ? '⏳ 部署中'
                       : version.deploy_info.deploy_status === 'failed' ? '❌ 部署失败'
                       : '⏸️ 待部署')
                    : '⏸️ 待部署';

                // 只有最新版本（index === 0）才显示按钮
                const isLatest = index === 0;
                
                return `
                    <div class="version-item">
                        <div class="version-item-header">
                            <div class="version-item-title">${escapeHtml(versionNumber)} ${isLatest ? '<span style="color: #28a745; font-size: 12px;">(最新)</span>' : ''}</div>
                            <div class="version-item-date">${versionDate}</div>
                        </div>
                        <div class="version-item-info">
                            <div class="version-item-pinid clickable" onclick="copyToClipboard('${version.pin_id}')" title="点击复制 PinID: ${version.pin_id}">
                                <strong>PinID:</strong> ${pinIdDisplay} 📋
                            </div>
                            <div style="display: flex; gap: 8px; align-items: center; justify-content: space-between; margin-top: 8px;">
                                <span style="font-size: 12px; color: #666;">${deployStatus}</span>
                                ${isLatest ? (isDeployed ? `
                                <div style="display: flex; gap: 8px;">
                                    <button class="btn-version-run" onclick="runVersionApp('${version.pin_id}')">
                                        运行
                                    </button>
                                    <button class="btn-version-download" onclick="downloadApp('${firstPinId}')">
                                        下载
                                    </button>
                                </div>
                                ` : `
                                <button class="btn-version-run" disabled title="该版本尚未部署完成">
                                    运行
                                </button>
                                `) : ''}
                            </div>
                        </div>
                    </div>
                `;
            }).join('');
        } else {
            throw new Error(data.message || '无法获取版本历史');
        }
    } catch (error) {
        console.error('Failed to load version history:', error);
        content.innerHTML = `
            <div class="empty-state">
                <div class="empty-state-icon">❌</div>
                <p>加载历史版本失败</p>
                <p style="font-size: 14px; margin-top: 10px;">${error.message}</p>
            </div>
        `;
        showNotification('加载历史版本失败: ' + error.message, 'error');
    }
}

// Close version history modal
function closeVersionHistoryModal(event) {
    if (event && event.target !== event.currentTarget) {
        return; // Don't close if clicking inside modal
    }
    const modal = document.getElementById('versionHistoryModal');
    modal.classList.remove('show');
}

// Run version app
async function runVersionApp(pinId) {
    try {
        // 检查应用部署状态
        const response = await fetch(`${API_BASE}/api/v1/metaapps/${pinId}`);
        const data = await response.json();
        
        if (data.code === 0 && data.data) {
            const app = data.data;
            const deployStatus = app.deploy_info ? app.deploy_info.deploy_status : null;
            
            if (deployStatus === 'completed') {
                // 部署成功，在新标签页中打开应用
                const appUrl = `${API_BASE}/${pinId}/index.html`;
                window.open(appUrl, '_blank');
                showNotification('正在打开应用...', 'success');
                // Close modal after opening app
                closeVersionHistoryModal();
            } else {
                showNotification('该版本尚未部署完成，无法运行', 'warning');
            }
        } else {
            showNotification('无法获取应用信息', 'error');
        }
    } catch (error) {
        console.error('Failed to run version app:', error);
        showNotification('运行应用失败: ' + error.message, 'error');
    }
}
