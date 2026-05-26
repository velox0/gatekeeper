(function () {
  // Check for reduced motion preferences
  var mq = window.matchMedia("(prefers-reduced-motion: reduce)");
  var animationsEnabled = !mq.matches;

  // Wait for DOM to be fully loaded
  window.addEventListener("DOMContentLoaded", function () {
    var origForm = document.getElementById("login-form");
    if (!origForm) return;

    var origUsername = document.getElementById("username");
    var origPassword = document.getElementById("password");
    if (!origUsername || !origPassword) return;

    // Get any existing error from the modern error banner
    var errorBanner = document.querySelector(".error-banner");
    var errorMessage = "";
    if (errorBanner) {
      // Extract clean error message text
      var span = errorBanner.querySelector("span");
      errorMessage = span ? span.textContent : errorBanner.textContent;
    }

    // Hide original elements
    var card = document.querySelector(".card");
    if (card) card.style.display = "none";
    var bg = document.querySelector(".bg-gradient");
    if (bg) bg.style.display = "none";
    var orbs = document.querySelectorAll(".orb");
    for (var i = 0; i < orbs.length; i++) {
      orbs[i].style.display = "none";
    }

    // Determine the AppName (or default)
    var logoTextEl = document.querySelector(".logo-text");
    var appName = logoTextEl ? logoTextEl.textContent : "Gatekeeper";

    // Create the Windows XP Welcome Screen Container
    var screenDiv = document.createElement("div");
    screenDiv.id = "winxp-screen";

    // 1. Top Bar
    var topBar = document.createElement("div");
    topBar.className = "xp-top-bar";
    topBar.innerHTML = '<div class="xp-top-inner"></div>';
    screenDiv.appendChild(topBar);

    // 2. Middle Panel
    var middlePanel = document.createElement("div");
    middlePanel.className = "xp-middle";

    var container = document.createElement("div");
    container.className = "xp-container";

    // Left Column: Brand & Logo
    var leftCol = document.createElement("div");
    leftCol.className = "xp-left-col";

    var logoArea = document.createElement("div");
    logoArea.className = "xp-logo-area";

    var brandContainer = document.createElement("div");
    brandContainer.className = "xp-brand-container";

    // Image Logo (loaded as Base64)
    var logoImg = 
      '<img class="xp-flag-svg" src="data:image/png;base64,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" alt="Logo" />';

    var brandText = document.createElement("div");
    brandText.className = "xp-brand-text";
    brandText.innerHTML = 
      '<span class="xp-brand-microsoft">Microsoft</span>' +
      '<span class="xp-brand-name">Windows<span style="font-weight:100; font-family:sans-serif; margin-left:2px;">xp</span></span>' +
      '<span class="xp-brand-edition">' + escapeHTML(appName) + ' Edition</span>';

    brandContainer.innerHTML = logoImg;
    brandContainer.appendChild(brandText);
    logoArea.appendChild(brandContainer);

    var welcomeMsg = document.createElement("div");
    welcomeMsg.className = "xp-welcome-msg";
    welcomeMsg.textContent = "To begin, type your username and password, then click the green button.";

    logoArea.appendChild(welcomeMsg);
    leftCol.appendChild(logoArea);
    container.appendChild(leftCol);

    // Divider
    var divider = document.createElement("div");
    divider.className = "xp-divider";
    container.appendChild(divider);

    // Right Column: Form Fields
    var rightCol = document.createElement("div");
    rightCol.className = "xp-right-col";

    var logonPanel = document.createElement("div");
    logonPanel.className = "xp-logon-panel";

    // User row/avatar header
    var userRow = document.createElement("div");
    userRow.className = "xp-user-row";
    
    // Chess avatar SVG
    var avatarSvg = 
      '<svg viewBox="0 0 100 100" class="xp-avatar-img" xmlns="http://www.w3.org/2000/svg">' +
        '<rect width="100" height="100" fill="url(#xp-avatar-grad)" />' +
        '<defs>' +
          '<radialGradient id="xp-avatar-grad" cx="50%" cy="50%" r="50%">' +
            '<stop offset="0%" stop-color="#55adff"/>' +
            '<stop offset="100%" stop-color="#194c8e"/>' +
          '</radialGradient>' +
        '</defs>' +
        '<!-- Pawn Silhouette -->' +
        '<circle cx="50" cy="27" r="10" fill="#ffffff" />' +
        '<path d="M 38,44 C 38,36 43,35 50,35 C 57,35 62,36 62,44 C 62,53 58,62 58,72 L 42,72 C 42,62 38,53 38,44 Z" fill="#ffffff" />' +
        '<rect x="36" y="74" width="28" height="5" rx="1.5" fill="#ffffff" />' +
        '<rect x="30" y="81" width="40" height="8" rx="2" fill="#ffffff" />' +
      '</svg>';

    userRow.innerHTML = 
      '<div class="xp-avatar-frame active">' + avatarSvg + '</div>' +
      '<div class="xp-user-info">' +
        '<span class="xp-user-name">Sign In</span>' +
        '<span class="xp-user-status">Enter credentials below</span>' +
      '</div>';
    
    logonPanel.appendChild(userRow);

    // Forms
    var formContainer = document.createElement("div");
    formContainer.className = "xp-form-container";

    // Username Group
    var userGroup = document.createElement("div");
    userGroup.className = "xp-input-group";
    userGroup.innerHTML = 
      '<label class="xp-input-label" for="xp-username-field">Username</label>' +
      '<div class="xp-input-wrapper">' +
        '<input type="text" id="xp-username-field" class="xp-input" autocomplete="username" />' +
      '</div>';
    formContainer.appendChild(userGroup);

    // Password Group
    var passGroup = document.createElement("div");
    passGroup.className = "xp-input-group";
    passGroup.innerHTML = 
      '<label class="xp-input-label" for="xp-password-field">Password</label>' +
      '<div class="xp-input-wrapper">' +
        '<input type="password" id="xp-password-field" class="xp-input" autocomplete="current-password" />' +
        '<button type="button" id="xp-go-button" class="xp-go-btn" title="Log On">' +
          '<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">' +
            '<path d="M5 12h14M12 5l7 7-7 7" stroke="#ffffff" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>' +
          '</svg>' +
        '</button>' +
      '</div>';
    formContainer.appendChild(passGroup);

    // Error Speech Balloon (if error exists)
    if (errorMessage) {
      var errorBalloon = document.createElement("div");
      errorBalloon.className = "xp-error-balloon";
      errorBalloon.id = "xp-error-bubble";
      errorBalloon.innerHTML = 
        '<div class="xp-error-header">' +
          '<svg class="xp-error-icon-svg" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">' +
            '<circle cx="12" cy="12" r="10" fill="#d32f2f"/>' +
            '<line x1="12" y1="8" x2="12" y2="12" stroke="#ffffff" stroke-width="2" stroke-linecap="round"/>' +
            '<circle cx="12" cy="16" r="1" fill="#ffffff"/>' +
          '</svg>' +
          '<span>Logon Message</span>' +
        '</div>' +
        '<div class="xp-error-message">' + escapeHTML(errorMessage) + '</div>';
      formContainer.appendChild(errorBalloon);
    }

    logonPanel.appendChild(formContainer);
    rightCol.appendChild(logonPanel);
    container.appendChild(rightCol);
    middlePanel.appendChild(container);
    screenDiv.appendChild(middlePanel);

    // 3. Bottom Bar
    var bottomBar = document.createElement("div");
    bottomBar.className = "xp-bottom-bar";
    
    var bottomInner = document.createElement("div");
    bottomInner.className = "xp-bottom-inner";
    
    // Shut down button
    var shutdownBtn = document.createElement("button");
    shutdownBtn.type = "button";
    shutdownBtn.className = "xp-shutdown-btn";
    shutdownBtn.innerHTML = 
      '<span class="xp-shutdown-icon">' +
        '<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">' +
          '<path d="M18.36 6.64a9 9 0 1 1-12.73 0M12 2v10" stroke-linecap="round"/>' +
        '</svg>' +
      '</span>' +
      '<span class="xp-shutdown-text">Turn off computer</span>';
    
    shutdownBtn.addEventListener("click", function () {
      // Redirect back or home as the shutdown action
      window.location.href = "/";
    });

    bottomInner.appendChild(shutdownBtn);

    var helpText = document.createElement("div");
    helpText.className = "xp-bottom-help";
    helpText.innerHTML = "After you log on, you will be redirected to your secure destination.<br/>Gatekeeper secures your web applications.";
    bottomInner.appendChild(helpText);

    bottomBar.appendChild(bottomInner);
    screenDiv.appendChild(bottomBar);

    // Add to page body
    document.body.insertBefore(screenDiv, document.body.firstChild);

    // Sync input fields
    var xpUserField = document.getElementById("xp-username-field");
    var xpPassField = document.getElementById("xp-password-field");
    var xpGoBtn = document.getElementById("xp-go-button");

    // Copy initial values (if browser auto-filled them)
    if (origUsername.value) {
      xpUserField.value = origUsername.value;
    }
    if (origPassword.value) {
      xpPassField.value = origPassword.value;
    }

    // Set up standard polling to capture browser autofill updates
    var autofillInterval = setInterval(function() {
      if (origUsername.value && !xpUserField.value) {
        xpUserField.value = origUsername.value;
      }
      if (origPassword.value && !xpPassField.value) {
        xpPassField.value = origPassword.value;
      }
    }, 100);
    // Stop polling after 3 seconds
    setTimeout(function() {
      clearInterval(autofillInterval);
    }, 3000);

    // Synchronize inputs back to hidden form
    xpUserField.addEventListener("input", function () {
      origUsername.value = xpUserField.value;
      hideErrorBalloon();
    });
    xpPassField.addEventListener("input", function () {
      origPassword.value = xpPassField.value;
      hideErrorBalloon();
    });

    // Form submission handlers
    function submitLogon() {
      origUsername.value = xpUserField.value;
      origPassword.value = xpPassField.value;
      
      // Basic client side validation
      if (!xpUserField.value) {
        showLocalError("Please enter your username.");
        xpUserField.focus();
        return;
      }
      if (!xpPassField.value) {
        showLocalError("Please enter your password.");
        xpPassField.focus();
        return;
      }
      
      // Submit the native form
      origForm.submit();
    }

    xpGoBtn.addEventListener("click", submitLogon);

    // Handle Enter keypress
    xpUserField.addEventListener("keydown", function (e) {
      if (e.key === "Enter") {
        e.preventDefault();
        xpPassField.focus();
      }
    });

    xpPassField.addEventListener("keydown", function (e) {
      if (e.key === "Enter") {
        e.preventDefault();
        submitLogon();
      }
    });

    // Error helper functions
    function hideErrorBalloon() {
      var bubble = document.getElementById("xp-error-bubble");
      if (bubble) {
        bubble.style.display = "none";
      }
    }

    function showLocalError(msg) {
      hideErrorBalloon();
      
      var bubble = document.getElementById("xp-error-bubble");
      if (!bubble) {
        bubble = document.createElement("div");
        bubble.className = "xp-error-balloon";
        bubble.id = "xp-error-bubble";
        formContainer.appendChild(bubble);
      }
      
      bubble.innerHTML = 
        '<div class="xp-error-header">' +
          '<svg class="xp-error-icon-svg" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">' +
            '<circle cx="12" cy="12" r="10" fill="#d32f2f"/>' +
            '<line x1="12" y1="8" x2="12" y2="12" stroke="#ffffff" stroke-width="2" stroke-linecap="round"/>' +
            '<circle cx="12" cy="16" r="1" fill="#ffffff"/>' +
          '</svg>' +
          '<span>Logon Message</span>' +
        '</div>' +
        '<div class="xp-error-message">' + escapeHTML(msg) + '</div>';
        
      bubble.style.display = "flex";
    }

    // Auto-focus username field on load
    xpUserField.focus();
  });

  // Simple HTML Escaping helper
  function escapeHTML(str) {
    if (!str) return "";
    return str
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#039;");
  }
})();
