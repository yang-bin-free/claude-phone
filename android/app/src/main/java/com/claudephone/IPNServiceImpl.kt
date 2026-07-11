package com.claudephone

import android.net.IpPrefix
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import java.net.InetAddress

interface IPNService {
    fun id(): String
    fun protect(fd: Int): Boolean
    fun newBuilder(): VPNServiceBuilder
    fun close()
    fun disconnectVPN()
    fun updateVpnStatus(connected: Boolean)
}

interface VPNServiceBuilder {
    fun setMTU(mtu: Int)
    fun addDNSServer(server: String)
    fun addSearchDomain(domain: String)
    fun addRoute(addr: String, prefixLen: Int)
    fun excludeRoute(addr: String, prefixLen: Int)
    fun addAddress(addr: String, prefixLen: Int)
    fun establish(): GoParcelFD?
}

interface GoParcelFD {
    fun detach(): Int
}

/**
 * IPNServiceImpl is the Kotlin-side VpnService bridge.
 *
 * ★ This is the core bridge: Go engine → Kotlin VpnService → tun fd → Go engine
 *
 */
class IPNServiceImpl : VpnService(), IPNService {

    private var uniqueId: String = "ipn-${System.currentTimeMillis()}"

    override fun id(): String = uniqueId

    // ★ IPNService.protect(fd) — anti-routing-loop callback
    // Go engine calls this for every socket it creates.
    // VpnService.protect() excludes the socket from the VPN tunnel.
    override fun protect(fd: Int): Boolean {
        return super.protect(fd)
    }

    override fun newBuilder(): VPNServiceBuilder {
        return VPNServiceBuilderImpl(this)
    }

    override fun close() { stopSelf() }
    override fun disconnectVPN() { stopSelf() }
    override fun updateVpnStatus(connected: Boolean) { /* TODO: notify UI */ }
}

/**
 * VPNServiceBuilderImpl wraps Android's VpnService.Builder.
 *
 * ★ When establish() is called, Android creates the tun interface
 * and returns a ParcelFileDescriptor. We detach the fd and pass it
 * to the Go engine — this is the tun fd that wireguard-go uses.
 *
 * Android API 33+ changed several VpnService.Builder method signatures:
 * - addRoute: (String, int) → (IpPrefix)
 * - addDnsServer: (String) → (InetAddress)
 * - addAddress: unchanged (still String + int)
 * - excludeRoute: new method, takes IpPrefix only
 */
class VPNServiceBuilderImpl(private val vpnService: VpnService) : VPNServiceBuilder {

    private val builder = vpnService.Builder()

    override fun setMTU(mtu: Int) { builder.setMtu(mtu) }

    override fun addDNSServer(server: String) {
        builder.addDnsServer(InetAddress.getByName(server))
    }

    override fun addSearchDomain(domain: String) {
        builder.addSearchDomain(domain)
    }

    override fun addRoute(addr: String, prefixLen: Int) {
        if (Build.VERSION.SDK_INT >= 33) {
            builder.addRoute(IpPrefix(InetAddress.getByName(addr), prefixLen))
        } else {
            builder.addRoute(addr, prefixLen)
        }
    }

    override fun excludeRoute(addr: String, prefixLen: Int) {
        if (Build.VERSION.SDK_INT >= 33) {
            builder.excludeRoute(IpPrefix(InetAddress.getByName(addr), prefixLen))
        }
    }

    override fun addAddress(addr: String, prefixLen: Int) {
        builder.addAddress(addr, prefixLen)
    }

    // ★ establish() — create the tun interface and return the fd
    override fun establish(): GoParcelFD? {
        val pfd = builder.establish() ?: return null
        return ParcelFDWrapper(pfd)
    }
}

/**
 * ParcelFDWrapper exposes detachFd() through the bridge interface.
 */
class ParcelFDWrapper(private val pfd: ParcelFileDescriptor) : GoParcelFD {
    override fun detach(): Int = pfd.detachFd()
}
