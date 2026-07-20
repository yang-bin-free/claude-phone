package com.claudephone

import android.app.Activity
import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Color
import android.net.Uri
import android.net.VpnService
import android.os.Bundle
import android.text.InputType
import android.view.ViewGroup
import android.webkit.WebChromeClient
import android.webkit.JavascriptInterface
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.TextView
import org.json.JSONObject

class MainActivity : Activity() {

    private lateinit var macAddress: EditText
    private lateinit var authKey: EditText
    private lateinit var deviceToken: EditText
    private lateinit var controlUrl: EditText
    private var currentWebView: WebView? = null
    private val speechController by lazy {
        OnDeviceSpeechController(this, object : VoiceCallbacks {
            override fun onText(text: String, final: Boolean) {
                currentWebView?.evaluateJavascript(
                    "window.codeAfar.setVoiceText(${JSONObject.quote(text)}, $final)", null
                )
            }

            override fun onState(state: VoiceState) {
                val (name, message) = when (state) {
                    VoiceState.Idle -> "idle" to ""
                    VoiceState.Listening -> "listening" to "正在聆听…"
                    VoiceState.Processing -> "processing" to "正在处理…"
                    is VoiceState.Unavailable -> "unavailable" to state.message
                    is VoiceState.Failed -> "failed" to state.message
                }
                currentWebView?.evaluateJavascript(
                    "window.codeAfar.setVoiceState(${JSONObject.quote(name)}, ${JSONObject.quote(message)})", null
                )
            }
        })
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        showPairingScreen()
    }

    private fun showPairingScreen() {
        currentWebView?.destroy()
        currentWebView = null
        val prefs = getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
        val container = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(48, 72, 48, 48)
            setBackgroundColor(Color.rgb(17, 19, 24))
        }
        container.addView(TextView(this).apply {
            text = "CodeAfar"
            textSize = 28f
            setTextColor(Color.WHITE)
        })
        container.addView(TextView(this).apply {
            text = "连接你的 Mac"
            textSize = 16f
            setTextColor(Color.LTGRAY)
            setPadding(0, 8, 0, 28)
        })

        macAddress = field("Mac 地址", prefs.getString(KEY_MAC_ADDRESS, "claude-mac:9876").orEmpty())
        authKey = field("Tailscale Auth Key（首次连接需要）", "")
        authKey.inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
        deviceToken = field("Device Token（Mac 端生成）", prefs.getString(KEY_DEVICE_TOKEN, "").orEmpty())
        deviceToken.inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
        controlUrl = field("Control URL（可选，Headscale）", prefs.getString(KEY_CONTROL_URL, "").orEmpty())
        container.addView(macAddress)
        container.addView(authKey)
        container.addView(deviceToken)
        container.addView(controlUrl)
        container.addView(Button(this).apply {
            text = "连接并打开聊天"
            setOnClickListener {
                val address = macAddress.text.toString().trim()
                if (address.isEmpty()) {
                    macAddress.error = "请输入 Mac 地址"
                    return@setOnClickListener
                }
                if (deviceToken.text.toString().trim().isEmpty()) {
                    deviceToken.error = "请在 Mac 端运行 codeafar-agent key 生成"
                    return@setOnClickListener
                }
                prefs.edit()
                    .putString(KEY_MAC_ADDRESS, address)
                    .putString(KEY_CONTROL_URL, controlUrl.text.toString().trim())
                    .putString(KEY_DEVICE_TOKEN, deviceToken.text.toString().trim())
                    .apply()
                requestVpnPermission()
            }
        }, LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT).apply {
            topMargin = 24
        })
        setContentView(container)
    }

    private fun field(hint: String, value: String) = EditText(this).apply {
        this.hint = hint
        setText(value)
        setTextColor(Color.WHITE)
        setHintTextColor(Color.GRAY)
        setSingleLine(true)
        setPadding(16, 16, 16, 16)
    }

    private fun requestVpnPermission() {
        val permissionIntent = VpnService.prepare(this)
        if (permissionIntent != null) {
            startActivityForResult(permissionIntent, VPN_REQUEST_CODE)
        } else {
            startVpnAndOpenChat()
        }
    }

    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == VPN_REQUEST_CODE && resultCode == RESULT_OK) {
            startVpnAndOpenChat()
        }
    }

    private fun startVpnAndOpenChat() {
        val prefs = getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
        val serviceIntent = Intent(this, IPNServiceImpl::class.java).apply {
            putExtra(IPNServiceImpl.EXTRA_HOSTNAME, "codeafar-android")
            putExtra(IPNServiceImpl.EXTRA_AUTH_KEY, authKey.text.toString().trim())
            putExtra(IPNServiceImpl.EXTRA_CONTROL_URL, controlUrl.text.toString().trim())
        }
        startForegroundService(serviceIntent)

        val authorizedDeviceToken = prefs.getString(KEY_DEVICE_TOKEN, "").orEmpty()
        val page = Uri.parse("file:///android_asset/chat/index.html").buildUpon()
            .appendQueryParameter("platform", "mobile")
            .appendQueryParameter("ws", webSocketURL(macAddress.text.toString()))
            .appendQueryParameter("deviceToken", authorizedDeviceToken)
            .appendQueryParameter("deviceName", android.os.Build.MODEL ?: "Android")
            .build()

        val webView = WebView(this).apply {
            settings.javaScriptEnabled = true
            settings.domStorageEnabled = true
            webViewClient = object : WebViewClient() {
                override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                    return !request.url.toString().startsWith("file:///android_asset/")
                }
            }
            webChromeClient = WebChromeClient()
            addJavascriptInterface(AndroidBridge(), "AndroidBridge")
            loadUrl(page.toString())
        }
        currentWebView = webView
        setContentView(webView)
    }

    private fun webSocketURL(value: String): String {
        val trimmed = value.trim().trimEnd('/')
        val base = when {
            trimmed.startsWith("ws://") || trimmed.startsWith("wss://") -> trimmed
            trimmed.startsWith("https://") -> "wss://${trimmed.removePrefix("https://")}"
            trimmed.startsWith("http://") -> "ws://${trimmed.removePrefix("http://")}"
            else -> "ws://$trimmed"
        }
        return if (base.endsWith("/ws")) base else "$base/ws"
    }

    @Suppress("DEPRECATION")
    override fun onBackPressed() {
        if (currentWebView != null) {
            disconnectAndShowSettings()
        } else {
            super.onBackPressed()
        }
    }

    private fun disconnectAndShowSettings() {
		speechController.destroy()
        stopService(Intent(this, IPNServiceImpl::class.java))
        showPairingScreen()
    }

    private inner class AndroidBridge {
        @JavascriptInterface
        fun openSettings() {
            runOnUiThread { disconnectAndShowSettings() }
        }

        @JavascriptInterface
        fun startVoice() {
            runOnUiThread {
                if (checkSelfPermission(Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED) {
                    speechController.toggle()
                } else {
                    requestPermissions(arrayOf(Manifest.permission.RECORD_AUDIO), AUDIO_PERMISSION_CODE)
                }
            }
        }
    }

    override fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode == AUDIO_PERMISSION_CODE && grantResults.firstOrNull() == PackageManager.PERMISSION_GRANTED) {
            speechController.toggle()
        } else if (requestCode == AUDIO_PERMISSION_CODE) {
            currentWebView?.evaluateJavascript(
                "window.codeAfar.setVoiceState(\"denied\", \"没有麦克风权限，请在系统设置中允许访问\")", null
            )
        }
    }

    override fun onDestroy() {
        speechController.destroy()
        currentWebView?.destroy()
        currentWebView = null
        super.onDestroy()
    }

    override fun onStop() {
        speechController.destroy()
        super.onStop()
    }

    companion object {
        private const val VPN_REQUEST_CODE = 100
        private const val PREFS_NAME = "claude_phone"
        private const val KEY_MAC_ADDRESS = "mac_address"
        private const val KEY_CONTROL_URL = "control_url"
        private const val KEY_DEVICE_TOKEN = "device_token"
        private const val AUDIO_PERMISSION_CODE = 102
    }
}
