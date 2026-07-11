package com.claudephone

import android.app.Activity
import android.content.Intent
import android.graphics.Color
import android.net.Uri
import android.net.VpnService
import android.os.Bundle
import android.text.InputType
import android.view.ViewGroup
import android.webkit.WebChromeClient
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.TextView
import java.util.UUID

class MainActivity : Activity() {

    private lateinit var macAddress: EditText
    private lateinit var authKey: EditText
    private lateinit var controlUrl: EditText

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        showPairingScreen()
    }

    private fun showPairingScreen() {
        val prefs = getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
        val container = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(48, 72, 48, 48)
            setBackgroundColor(Color.rgb(17, 19, 24))
        }
        container.addView(TextView(this).apply {
            text = "Claude Phone"
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
        controlUrl = field("Control URL（可选，Headscale）", prefs.getString(KEY_CONTROL_URL, "").orEmpty())
        container.addView(macAddress)
        container.addView(authKey)
        container.addView(controlUrl)
        container.addView(Button(this).apply {
            text = "连接并打开聊天"
            setOnClickListener {
                val address = macAddress.text.toString().trim()
                if (address.isEmpty()) {
                    macAddress.error = "请输入 Mac 地址"
                    return@setOnClickListener
                }
                prefs.edit()
                    .putString(KEY_MAC_ADDRESS, address)
                    .putString(KEY_CONTROL_URL, controlUrl.text.toString().trim())
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
            putExtra(IPNServiceImpl.EXTRA_HOSTNAME, "claude-phone-android")
            putExtra(IPNServiceImpl.EXTRA_AUTH_KEY, authKey.text.toString().trim())
            putExtra(IPNServiceImpl.EXTRA_CONTROL_URL, controlUrl.text.toString().trim())
        }
        startService(serviceIntent)

        val deviceToken = prefs.getString(KEY_DEVICE_TOKEN, null) ?: UUID.randomUUID().toString().also {
            prefs.edit().putString(KEY_DEVICE_TOKEN, it).apply()
        }
        val page = Uri.parse("file:///android_asset/chat/index.html").buildUpon()
            .appendQueryParameter("platform", "mobile")
            .appendQueryParameter("ws", webSocketURL(macAddress.text.toString()))
            .appendQueryParameter("deviceToken", "android-$deviceToken")
            .appendQueryParameter("deviceName", android.os.Build.MODEL ?: "Android")
            .build()

        setContentView(WebView(this).apply {
            settings.javaScriptEnabled = true
            settings.domStorageEnabled = true
            webViewClient = WebViewClient()
            webChromeClient = WebChromeClient()
            loadUrl(page.toString())
        })
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

    companion object {
        private const val VPN_REQUEST_CODE = 100
        private const val PREFS_NAME = "claude_phone"
        private const val KEY_MAC_ADDRESS = "mac_address"
        private const val KEY_CONTROL_URL = "control_url"
        private const val KEY_DEVICE_TOKEN = "device_token"
    }
}
