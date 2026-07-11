package com.claudephone

import android.app.Activity
import android.content.Intent
import android.net.VpnService
import android.os.Bundle
import android.widget.TextView
import androidlib.Androidlib

class MainActivity : Activity() {

    companion object {
        private const val VPN_REQUEST_CODE = 100
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // P0a verification: basic gomobile bind
        val greeting = Androidlib.hello("阿彬")

        // P0b verification: show that tailscale package is available
        val tv = TextView(this)
        tv.text = """
            $greeting

            P0b: tailscale package bound ✓

            Classes available:
            - EngineBackend
            - IPNService (interface → Kotlin implements)
            - VPNServiceBuilder (interface → Kotlin implements)
            - VPNFacade, VPNServiceState

            [Tap to request VPN permission]
        """.trimIndent()
        tv.textSize = 16f
        tv.setOnClickListener { requestVpnPermission() }
        setContentView(tv)
    }

    private fun requestVpnPermission() {
        val intent = VpnService.prepare(this)
        if (intent != null) {
            startActivityForResult(intent, VPN_REQUEST_CODE)
        } else {
            // VPN permission already granted
            startVpnAndEngine()
        }
    }

    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == VPN_REQUEST_CODE && resultCode == RESULT_OK) {
            startVpnAndEngine()
        }
    }

    private fun startVpnAndEngine() {
        // ★ P0b core: start IPNService (VpnService subclass)
        val intent = Intent(this, IPNServiceImpl::class.java)
        startService(intent)

        // Show result
        val tv = TextView(this)
        tv.text = "VPN Service started! Engine integration pending P0c."
        tv.textSize = 18f
        setContentView(tv)
    }
}
