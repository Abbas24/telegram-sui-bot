powershell
$ErrorActionPreference = "Stop"

$BaseUrl = "https://github.com/alireza0/s-ui/wiki"
$Output = Join-Path (Get-Location) "s-ui-wiki-offline"

$Pages = @(
    @{
        Name = "Home"
        Url  = "$BaseUrl"
    },
    @{
        Name = "Subscription-Service"
        Url  = "$BaseUrl/Subscription-Service"
    },
    @{
        Name = "Subscription-JSON-Template"
        Url  = "$BaseUrl/Subscription-JSON-Template"
    },
    @{
        Name = "Subscription-Clash-Template"
        Url  = "$BaseUrl/Subscription-Clash-Template"
    },
    @{
        Name = "API-Documentation"
        Url  = "$BaseUrl/API-Documentation"
    },
    @{
        Name = "Configuration-Objects"
        Url  = "$BaseUrl/Configuration-Objects"
    },
    @{
        Name = "Settings-Reference"
        Url  = "$BaseUrl/Settings-Reference"
    }
)

Write-Host ""
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "        S-UI Wiki Offline Downloader" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""

# Create output directories
New-Item -ItemType Directory -Force -Path $Output | Out-Null
New-Item -ItemType Directory -Force -Path "$Output\pages" | Out-Null
New-Item -ItemType Directory -Force -Path "$Output\assets" | Out-Null

# CSS for the offline documentation
$Css = @"
* {
    box-sizing: border-box;
}

body {
    margin: 0;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Arial, sans-serif;
    background: #ffffff;
    color: #24292f;
    line-height: 1.6;
}

.layout {
    display: flex;
    min-height: 100vh;
}

.sidebar {
    width: 280px;
    min-width: 280px;
    background: #f6f8fa;
    border-right: 1px solid #d0d7de;
    padding: 24px 16px;
    position: fixed;
    top: 0;
    bottom: 0;
    overflow-y: auto;
}

.sidebar h1 {
    font-size: 20px;
    margin: 0 0 20px 8px;
}

.sidebar h2 {
    font-size: 13px;
    text-transform: uppercase;
    color: #656d76;
    margin: 22px 8px 8px;
}

.sidebar a {
    display: block;
    padding: 7px 10px;
    margin: 2px 0;
    border-radius: 6px;
    color: #24292f;
    text-decoration: none;
    font-size: 14px;
}

.sidebar a:hover {
    background: #eaeef2;
}

.content {
    margin-left: 280px;
    width: calc(100% - 280px);
    max-width: 1100px;
    padding: 40px 60px 80px;
}

.content h1 {
    font-size: 32px;
    border-bottom: 1px solid #d8dee4;
    padding-bottom: 10px;
}

.content h2 {
    font-size: 24px;
    border-bottom: 1px solid #d8dee4;
    padding-bottom: 6px;
}

.content h3 {
    font-size: 20px;
}

.content a {
    color: #0969da;
}

pre {
    background: #f6f8fa;
    padding: 16px;
    overflow-x: auto;
    border-radius: 6px;
}

code {
    font-family: Consolas, "Courier New", monospace;
}

:not(pre) > code {
    background: #eff1f3;
    padding: 2px 5px;
    border-radius: 4px;
}

blockquote {
    border-left: 4px solid #d0d7de;
    padding-left: 16px;
    color: #656d76;
}

table {
    border-collapse: collapse;
    width: 100%;
    margin: 16px 0;
}

th, td {
    border: 1px solid #d0d7de;
    padding: 8px 12px;
    text-align: left;
}

th {
    background: #f6f8fa;
}

img {
    max-width: 100%;
    height: auto;
}

.search {
    width: 100%;
    padding: 9px 10px;
    border: 1px solid #d0d7de;
    border-radius: 6px;
    margin-bottom: 15px;
    font-size: 14px;
}

@media (max-width: 800px) {
    .sidebar {
        position: static;
        width: 100%;
        min-width: 0;
    }

    .layout {
        display: block;
    }

    .content {
        margin-left: 0;
        width: 100%;
        padding: 25px;
    }
}
"@

Set-Content -Path "$Output\assets\style.css" -Value $Css -Encoding UTF8

# Download a page
function Download-Page {
    param(
        [string]$Name,
        [string]$Url
    )

    Write-Host "Downloading $Name..." -ForegroundColor Yellow

    try {
        $Response = Invoke-WebRequest `
            -Uri $Url `
            -UseBasicParsing `
            -Headers @{
                "User-Agent" = "Mozilla/5.0"
                "Accept" = "text/html"
            }

        $Html = $Response.Content

        if ([string]::IsNullOrWhiteSpace($Html)) {
            throw "Empty response"
        }

        # Save original GitHub HTML as a backup
        Set-Content `
            -Path "$Output\pages\$Name-github.html" `
            -Value $Html `
            -Encoding UTF8

        Write-Host "  OK" -ForegroundColor Green
        return $Html
    }
    catch {
        Write-Host "  FAILED: $($_.Exception.Message)" -ForegroundColor Red
        return $null
    }
}

$Downloaded = @{}

foreach ($Page in $Pages) {
    $Downloaded[$Page.Name] = Download-Page `
        -Name $Page.Name `
        -Url $Page.Url

    Start-Sleep -Milliseconds 500
}

# Navigation
$Nav = @"
<nav class="sidebar">
    <h1>S-UI Wiki</h1>

    <input
        id="search"
        class="search"
        type="text"
        placeholder="Search wiki..."
        onkeyup="searchWiki()"
    />

    <h2>General</h2>
    <a href="index.html">Home</a>

    <h2>Subscriptions</h2>
    <a href="pages/Subscription-Service.html">Subscription Service</a>
    <a href="pages/Subscription-JSON-Template.html">Subscription JSON Template</a>
    <a href="pages/Subscription-Clash-Template.html">Subscription Clash Template</a>

    <h2>Automation</h2>
    <a href="pages/API-Documentation.html">API Documentation</a>
    <a href="pages/Configuration-Objects.html">Configuration Objects</a>
    <a href="pages/Settings-Reference.html">Settings Reference</a>

    <h2>Source</h2>
    <a href="https://github.com/alireza0/s-ui/wiki" target="_blank">
        GitHub Wiki
    </a>
</nav>
"@

# JavaScript for local search
$Js = @"
function searchWiki() {
    const input = document.getElementById("search");
    const filter = input.value.toLowerCase();

    const links = document.querySelectorAll(".sidebar a");

    links.forEach(function(link) {
        const text = link.textContent.toLowerCase();

        if (text.includes(filter) || filter === "") {
            link.style.display = "block";
        } else {
            link.style.display = "none";
        }
    });
}
"@

Set-Content -Path "$Output\assets\search.js" -Value $Js -Encoding UTF8

# Extract the wiki article from GitHub HTML.
#
# GitHub's HTML structure changes periodically, so several
# patterns are attempted.
function Extract-Article {
    param(
        [string]$Html
    )

    # Pattern 1: markdown-body
    $Pattern = '(?s)<div[^>]*class="[^"]*markdown-body[^"]*"[^>]*>(.*?)</div>\s*</div>'

    $Match = [regex]::Match(
        $Html,
        $Pattern,
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )

    if ($Match.Success) {
        return $Match.Groups[1].Value
    }

    # Pattern 2: article element
    $Pattern = '(?s)<article[^>]*>(.*?)</article>'

    $Match = [regex]::Match(
        $Html,
        $Pattern,
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )

    if ($Match.Success) {
        return $Match.Groups[1].Value
    }

    # Pattern 3: main content
    $Pattern = '(?s)<main[^>]*>(.*?)</main>'

    $Match = [regex]::Match(
        $Html,
        $Pattern,
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )

    if ($Match.Success) {
        return $Match.Groups[1].Value
    }

    return $null
}

# Fix internal wiki links so they work offline
function Fix-Links {
    param(
        [string]$Html
    )

    $Html = $Html -replace `
        'href="https://github\.com/alireza0/s-ui/wiki/"', `
        'href="../index.html"'

    $Html = $Html -replace `
        'href="https://github\.com/alireza0/s-ui/wiki/Subscription-Service"', `
        'href="Subscription-Service.html"'

    $Html = $Html -replace `
        'href="https://github\.com/alireza0/s-ui/wiki/Subscription-JSON-Template"', `
        'href="Subscription-JSON-Template.html"'

    $Html = $Html -replace `
        'href="https://github\.com/alireza0/s-ui/wiki/Subscription-Clash-Template"', `
        'href="Subscription-Clash-Template.html"'

    $Html = $Html -replace `
        'href="https://github\.com/alireza0/s-ui/wiki/API-Documentation"', `
        'href="API-Documentation.html"'

    $Html = $Html -replace `
        'href="https://github\.com/alireza0/s-ui/wiki/Configuration-Objects"', `
        'href="Configuration-Objects.html"'

    $Html = $Html -replace `
        'href="https://github\.com/alireza0/s-ui/wiki/Settings-Reference"', `
        'href="Settings-Reference.html"'

    # Relative wiki links
    $Html = $Html -replace `
        'href="/alireza0/s-ui/wiki/Subscription-Service"', `
        'href="Subscription-Service.html"'

    $Html = $Html -replace `
        'href="/alireza0/s-ui/wiki/Subscription-JSON-Template"', `
        'href="Subscription-JSON-Template.html"'

    $Html = $Html -replace `
        'href="/alireza0/s-ui/wiki/Subscription-Clash-Template"', `
        'href="Subscription-Clash-Template.html"'

    $Html = $Html -replace `
        'href="/alireza0/s-ui/wiki/API-Documentation"', `
        'href="API-Documentation.html"'

    $Html = $Html -replace `
        'href="/alireza0/s-ui/wiki/Configuration-Objects"', `
        'href="Configuration-Objects.html"'

    $Html = $Html -replace `
        'href="/alireza0/s-ui/wiki/Settings-Reference"', `
        'href="Settings-Reference.html"'

    return $Html
}

# Generate a page
function Generate-OfflinePage {
    param(
        [string]$Name,
        [string]$Html,
        [string]$Title,
        [string]$OutputFile
    )

    if ([string]::IsNullOrWhiteSpace($Html)) {
        Write-Host "  Skipping $Name - no downloaded content" -ForegroundColor Red
        return
    }

    $Article = Extract-Article $Html

    if ([string]::IsNullOrWhiteSpace($Article)) {
        Write-Host "  Could not extract article for $Name" -ForegroundColor Red
        return
    }

    $Article = Fix-Links $Article

    $Page = @"
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>$Title - S-UI Wiki</title>
<link rel="stylesheet" href="../assets/style.css">
</head>

<body>

<div class="layout">

$Nav

<main class="content">

$Article

</main>

</div>

<script src="../assets/search.js"></script>

</body>
</html>
"@

    Set-Content `
        -Path "$Output\pages\$OutputFile" `
        -Value $Page `
        -Encoding UTF8

    Write-Host "  Generated $OutputFile" -ForegroundColor Green
}

foreach ($Page in $Pages) {
    if ($Page.Name -eq "Home") {
        continue
    }

    Generate-OfflinePage `
        -Name $Page.Name `
        -Html $Downloaded[$Page.Name] `
        -Title $Page.Name `
        -OutputFile "$($Page.Name).html"
}

# Generate Home page
$HomeArticle = Extract-Article $Downloaded["Home"]

if ($HomeArticle) {
    $HomeArticle = Fix-Links $HomeArticle
}
else {
    $HomeArticle = @"
<h1>S-UI Wiki</h1>

<p>Offline documentation for S-UI.</p>

<h2>Documentation</h2>

<ul>
<li><a href="pages/Subscription-Service.html">Subscription Service</a></li>
<li><a href="pages/Subscription-JSON-Template.html">Subscription JSON Template</a></li>
<li><a href="pages/Subscription-Clash-Template.html">Subscription Clash Template</a></li>
<li><a href="pages/API-Documentation.html">API Documentation</a></li>
<li><a href="pages/Configuration-Objects.html">Configuration Objects</a></li>
<li><a href="pages/Settings-Reference.html">Settings Reference</a></li>
</ul>
"@
}

$HomePage = @"
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>S-UI Wiki</title>
<link rel="stylesheet" href="assets/style.css">
</head>

<body>

<div class="layout">

<nav class="sidebar">
    <h1>S-UI Wiki</h1>

    <input
        id="search"
        class="search"
        type="text"
        placeholder="Search wiki..."
        onkeyup="searchWiki()"
    />

    <h2>General</h2>
    <a href="index.html">Home</a>

    <h2>Subscriptions</h2>
    <a href="pages/Subscription-Service.html">Subscription Service</a>
    <a href="pages/Subscription-JSON-Template.html">Subscription JSON Template</a>
    <a href="pages/Subscription-Clash-Template.html">Subscription Clash Template</a>

    <h2>Automation</h2>
    <a href="pages/API-Documentation.html">API Documentation</a>
    <a href="pages/Configuration-Objects.html">Configuration Objects</a>
    <a href="pages/Settings-Reference.html">Settings Reference</a>
</nav>

<main class="content">

$HomeArticle

</main>

</div>

<script src="assets/search.js"></script>

</body>
</html>
"@

Set-Content `
    -Path "$Output\index.html" `
    -Value $HomePage `
    -Encoding UTF8

Write-Host ""
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "Download complete." -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Offline wiki:" -ForegroundColor White
Write-Host "$Output\index.html" -ForegroundColor Yellow
Write-Host ""
Write-Host "Open it with:" -ForegroundColor White
Write-Host "start `"$Output\index.html`"" -ForegroundColor Yellow
Write-Host ""

