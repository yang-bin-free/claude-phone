//go:build android

// SPDX-License-Identifier: MIT

package tailscale

// AppContext provides hooks into functionality implemented on the Kotlin side.
// Gomobile generates a Java interface from this Go interface; Kotlin implements it.
type AppContext interface {
	// Log logs the given tag and logLine
	Log(tag, logLine string)

	// GetInterfacesAsJson returns a JSON representation of network interfaces.
	GetInterfacesAsJson() (string, error)

	// GetSDKInt returns the Android SDK_INT version.
	GetSDKInt() (int, error)

	// GetOSVersion returns the Android OS version string.
	GetOSVersion() (string, error)
}

// IPNService corresponds to Kotlin's IPNService (VpnService subclass).
// Gomobile generates a Java interface; Kotlin implements it.
type IPNService interface {
	// ID returns a unique ID for this VPN service instance.
	ID() string

	// Protect protects a socket fd from being captured by the VPN tunnel.
	// This is the critical anti-routing-loop callback.
	// It calls Android VpnService.protect(fd) which excludes the socket
	// from the VPN tunnel, preventing routing loops.
	Protect(fd int32) bool

	// NewBuilder creates a new VpnService.Builder for configuring the tunnel.
	NewBuilder() VPNServiceBuilder

	// Close shuts down the VPN service.
	Close()

	// DisconnectVPN disconnects the VPN.
	DisconnectVPN()

	// UpdateVpnStatus updates the VPN connection status.
	UpdateVpnStatus(bool)
}

// VPNServiceBuilder corresponds to Android's VpnService.Builder.
type VPNServiceBuilder interface {
	SetMTU(int32) error
	AddDNSServer(string) error
	AddSearchDomain(string) error
	AddRoute(string, int32) error
	ExcludeRoute(string, int32) error
	AddAddress(string, int32) error
	Establish() (ParcelFileDescriptor, error)
}

// ParcelFileDescriptor corresponds to Android's ParcelFileDescriptor.
type ParcelFileDescriptor interface {
	Detach() (int32, error)
}
