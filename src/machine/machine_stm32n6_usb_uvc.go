//go:build stm32n6

package machine

// UVC 1.1 single-format YUY2 webcam on top of the N6 DWC2 device driver.
//
// Fixed configuration (keeps descriptors and probe/commit trivial):
//
//   Format:     YUY2 / YUYV (uncompressed 16 bpp)
//   Resolution: 160 × 120
//   Frame rate: 15 fps (dwFrameInterval = 666_666 × 100 ns)
//   Endpoint:   iso IN on EP1 at 512 B/microframe (one packet per µframe)
//
// Descriptor-wise this is a UVC-only device (CDC is replaced, not
// composed). Composite UVC+CDC comes later. Host-side:
//   $ v4l2-ctl --list-devices
//   NUCLEO-N657 (usb-...)
//       /dev/video0
//   $ ffplay -f v4l2 -input_format yuyv422 -video_size 160x120 /dev/video0
//
// Frame source is a synthetic test pattern generated per-packet — no
// frame buffer is allocated. Replace uvcPixel() when wiring a real
// camera (DCMIPP → UVC later).

import (
	"machine/usb"
)

// ---------------------------------------------------------------------------
// UVC class constants (from USB Video Class 1.1 spec).
// ---------------------------------------------------------------------------

const (
	uvcCC_VIDEO            = 0x0E
	uvcSC_VIDEOCONTROL     = 0x01
	uvcSC_VIDEOSTREAMING   = 0x02
	uvcSC_IFACE_COLLECTION = 0x03

	// Descriptor types
	uvcCS_INTERFACE = 0x24
	uvcCS_ENDPOINT  = 0x25

	// VideoControl interface descriptor subtypes
	uvcVC_HEADER          = 0x01
	uvcVC_INPUT_TERMINAL  = 0x02
	uvcVC_OUTPUT_TERMINAL = 0x03
	uvcVC_SELECTOR_UNIT   = 0x04
	uvcVC_PROCESSING_UNIT = 0x05

	// VideoStreaming interface descriptor subtypes
	uvcVS_INPUT_HEADER        = 0x01
	uvcVS_FORMAT_UNCOMPRESSED = 0x04
	uvcVS_FRAME_UNCOMPRESSED  = 0x05
	uvcVS_COLORFORMAT         = 0x0D

	// Terminal types
	uvcITT_CAMERA   = 0x0201
	uvcTT_STREAMING = 0x0101

	// Class-specific request codes (bRequest)
	uvcSET_CUR  = 0x01
	uvcGET_CUR  = 0x81
	uvcGET_MIN  = 0x82
	uvcGET_MAX  = 0x83
	uvcGET_RES  = 0x84
	uvcGET_LEN  = 0x85
	uvcGET_INFO = 0x86
	uvcGET_DEF  = 0x87

	// VS control selectors (put in wValue HIGH byte of a class request)
	uvcVS_PROBE_CONTROL  = 0x01
	uvcVS_COMMIT_CONTROL = 0x02

	// Fixed stream geometry
	uvcWidth        = 160
	uvcHeight       = 120
	uvcBPP          = 2                             // YUY2 = 16 bpp
	uvcFrameBytes   = uvcWidth * uvcHeight * uvcBPP // 38_400
	uvcFrameIntvl   = 666_666                       // 15 fps, 100ns units
	uvcIsoEP        = 1                             // EP1 IN
	uvcIsoMPS       = 512                           // HS iso packet size
	uvcPayloadBytes = uvcIsoMPS - 2                 // minus 2-byte UVC header

	// VS interface number used in all descriptors + SET_INTERFACE hook.
	uvcVCInterface = 0
	uvcVSInterface = 1

	// Terminal / unit IDs.
	uvcTermCamera     = 1
	uvcUnitProcessing = 2
	uvcTermStreaming  = 3
)

// YUY2 format GUID: 32595559-0000-0010-8000-00AA00389B71
// (little-endian — first 4 bytes are ASCII "YUY2" = 59 55 59 32 reversed to
// read "YUY2" in memory order).
var uvcYUY2GUID = [16]byte{
	0x59, 0x55, 0x59, 0x32,
	0x00, 0x00, 0x10, 0x00,
	0x80, 0x00, 0x00, 0xAA,
	0x00, 0x38, 0x9B, 0x71,
}

// ---------------------------------------------------------------------------
// Descriptors.
//
// Device descriptor: declares IAD device class (0xEF / 0x02 / 0x01) so the
// host knows to look inside for an IAD even though we only have one
// function. This keeps descriptors on a common path if/when we go
// composite later.
// ---------------------------------------------------------------------------

var uvcDeviceDesc = [18]byte{
	18,         // bLength
	0x01,       // bDescriptorType = DEVICE
	0x00, 0x02, // bcdUSB = 2.00
	0xEF,       // bDeviceClass = Miscellaneous (IAD)
	0x02,       // bDeviceSubClass = Common Class
	0x01,       // bDeviceProtocol = Interface Association
	64,         // bMaxPacketSize0
	0x83, 0x04, // idVendor = 0x0483 (ST)
	0x41, 0x57, // idProduct = 0x5741 (+1 from our CDC PID)
	0x00, 0x01, // bcdDevice = 1.00
	1, // iManufacturer
	2, // iProduct
	3, // iSerialNumber
	1, // bNumConfigurations
}

// Config descriptor (total length = wTotalLength, which we patch in at
// init time so we never have to re-count bytes by hand when shifting
// things around).
var uvcConfigDesc = []byte{
	// --- Configuration descriptor ------------------------------------- 9
	9, 0x02,
	0x00, 0x00, // wTotalLength (patched in init)
	0x02, // bNumInterfaces (VC + VS)
	0x01, // bConfigurationValue
	0x00, // iConfiguration
	0x80, // bmAttributes = bus powered
	50,   // bMaxPower = 100 mA

	// --- Interface Association Descriptor (VC + VS) ------------------- 8
	8, 0x0B,
	uvcVCInterface,         // bFirstInterface
	2,                      // bInterfaceCount (VC + VS)
	uvcCC_VIDEO,            // bFunctionClass
	uvcSC_IFACE_COLLECTION, // bFunctionSubClass
	0x00,                   // bFunctionProtocol
	0,                      // iFunction

	// --- VideoControl interface -------------------------------------- 9
	9, 0x04,
	uvcVCInterface, // bInterfaceNumber
	0,              // bAlternateSetting
	0,              // bNumEndpoints (no endpoints — we don't use an
	//               interrupt status EP)
	uvcCC_VIDEO,        // bInterfaceClass
	uvcSC_VIDEOCONTROL, // bInterfaceSubClass
	0,                  // bInterfaceProtocol
	0,                  // iInterface

	// --- VC Class-specific VC Header --------------------------------- 13
	13, uvcCS_INTERFACE, uvcVC_HEADER,
	0x10, 0x01, // bcdUVC = 1.10
	0x27, 0x00, // wTotalLength = sum of VC sub-descriptors (patched)
	0x80, 0x8D, 0x5B, 0x00, // dwClockFrequency = 6 MHz (informational)
	1,              // bInCollection = 1 streaming iface
	uvcVSInterface, // baInterfaceNr[0]

	// --- VC Input Terminal (Camera) --------------------------------- 17
	17, uvcCS_INTERFACE, uvcVC_INPUT_TERMINAL,
	uvcTermCamera, // bTerminalID
	0x01, 0x02,    // wTerminalType = ITT_CAMERA (0x0201)
	0,          // bAssocTerminal
	0,          // iTerminal
	0x00, 0x00, // wObjectiveFocalLengthMin
	0x00, 0x00, // wObjectiveFocalLengthMax
	0x00, 0x00, // wOcularFocalLength
	2,          // bControlSize
	0x00, 0x00, // bmControls (no controls)

	// --- VC Processing Unit ---------------------------------------- 11
	11, uvcCS_INTERFACE, uvcVC_PROCESSING_UNIT,
	uvcUnitProcessing, // bUnitID
	uvcTermCamera,     // bSourceID (= camera)
	0x00, 0x00,        // wMaxMultiplier
	2,          // bControlSize
	0x00, 0x00, // bmControls (no controls)
	0, // iProcessing — required by UVC 1.0 §3.7.2.5

	// --- VC Output Terminal (Streaming) ------------------------------ 9
	9, uvcCS_INTERFACE, uvcVC_OUTPUT_TERMINAL,
	uvcTermStreaming, // bTerminalID
	0x01, 0x01,       // wTerminalType = TT_STREAMING (0x0101)
	0,                 // bAssocTerminal
	uvcUnitProcessing, // bSourceID (= PU)
	0,                 // iTerminal

	// --- VideoStreaming interface, alt 0 (zero-bandwidth) ------------ 9
	9, 0x04,
	uvcVSInterface,
	0, // bAlternateSetting = 0
	0, // bNumEndpoints (zero — required for host enumeration to work)
	uvcCC_VIDEO,
	uvcSC_VIDEOSTREAMING,
	0, 0,

	// --- VS Input Header --------------------------------------------- 14
	14, uvcCS_INTERFACE, uvcVS_INPUT_HEADER,
	1,          // bNumFormats
	0x5D, 0x00, // wTotalLength (sum of VS descs after this; patched)
	0x81,             // bEndpointAddress = EP1 IN
	0x00,             // bmInfo
	uvcTermStreaming, // bTerminalLink
	0,                // bStillCaptureMethod
	0,                // bTriggerSupport
	0,                // bTriggerUsage
	1,                // bControlSize
	0x00,             // bmaControls[0] (no VS controls)

	// --- VS Format Uncompressed (YUY2) ------------------------------- 27
	27, uvcCS_INTERFACE, uvcVS_FORMAT_UNCOMPRESSED,
	1,                      // bFormatIndex
	1,                      // bNumFrameDescriptors
	0x59, 0x55, 0x59, 0x32, // guidFormat — YUY2
	0x00, 0x00, 0x10, 0x00,
	0x80, 0x00, 0x00, 0xAA,
	0x00, 0x38, 0x9B, 0x71,
	16, // bBitsPerPixel
	1,  // bDefaultFrameIndex
	0,  // bAspectRatioX
	0,  // bAspectRatioY
	0,  // bmInterlaceFlags
	0,  // bCopyProtect

	// --- VS Frame Uncompressed --------------------------------------- 30
	30, uvcCS_INTERFACE, uvcVS_FRAME_UNCOMPRESSED,
	1,    // bFrameIndex
	0x00, // bmCapabilities
	// wWidth / wHeight (160 × 120)
	byte(uvcWidth & 0xFF), byte(uvcWidth >> 8),
	byte(uvcHeight & 0xFF), byte(uvcHeight >> 8),
	// dwMinBitRate = frame * fps * 8  (only one rate, so min == max)
	// = 38400 * 15 * 8 = 4_608_000 = 0x0046_5000
	0x00, 0x50, 0x46, 0x00,
	// dwMaxBitRate
	0x00, 0x50, 0x46, 0x00,
	// dwMaxVideoFrameBufferSize = uvcFrameBytes = 38_400 = 0x9600
	0x00, 0x96, 0x00, 0x00,
	// dwDefaultFrameInterval = 666666 = 0xA2C2A
	0x2A, 0x2C, 0x0A, 0x00,
	// bFrameIntervalType = 1 (one discrete interval)
	1,
	// dwFrameInterval[0] = 666666
	0x2A, 0x2C, 0x0A, 0x00,

	// --- VS Color Matching ------------------------------------------- 6
	6, uvcCS_INTERFACE, uvcVS_COLORFORMAT,
	1, // bColorPrimaries = BT.709
	1, // bTransferCharacteristics = BT.709
	4, // bMatrixCoefficients = SMPTE 170M (BT.601)

	// --- VideoStreaming interface, alt 1 (streaming) ----------------- 9
	9, 0x04,
	uvcVSInterface,
	1, // bAlternateSetting = 1
	1, // bNumEndpoints
	uvcCC_VIDEO,
	uvcSC_VIDEOSTREAMING,
	0, 0,

	// --- Endpoint: iso IN, EP1 --------------------------------------- 7
	7, 0x05,
	0x81,       // bEndpointAddress = EP1 IN
	0x05,       // bmAttributes = iso, async, data
	0x00, 0x02, // wMaxPacketSize = 512 (MCNT=1 implicit in bits 13:12=00)
	1, // bInterval = 1 microframe (HS)
}

// ---------------------------------------------------------------------------
// Probe / commit state. UVC 1.1 = 34-byte struct.
// ---------------------------------------------------------------------------

var uvcProbe = [34]byte{
	0x00, 0x00, // bmHint
	1, // bFormatIndex
	1, // bFrameIndex
	// dwFrameInterval = 666666
	0x2A, 0x2C, 0x0A, 0x00,
	0x00, 0x00, // wKeyFrameRate
	0x00, 0x00, // wPFrameRate
	0x00, 0x00, // wCompQuality
	0x00, 0x00, // wCompWindowSize
	0x00, 0x00, // wDelay
	// dwMaxVideoFrameSize = 38400 = 0x9600
	0x00, 0x96, 0x00, 0x00,
	// dwMaxPayloadTransferSize = 512 (one iso packet)
	0x00, 0x02, 0x00, 0x00,
	// dwClockFrequency = 6 MHz
	0x80, 0x8D, 0x5B, 0x00,
	0x00, // bmFramingInfo
	0x00, // bPreferedVersion
	0x00, // bMinVersion
	0x00, // bMaxVersion
}

// Fixed-offset patch helper — used at init() to fill in the descriptor
// wTotalLength fields after we assemble the slice.
func uvcPatchLengths() {
	totalLen := uint16(len(uvcConfigDesc))
	uvcConfigDesc[2] = byte(totalLen)
	uvcConfigDesc[3] = byte(totalLen >> 8)

	// VC wTotalLength = VC header + IT + PU + OT = 13 + 17 + 11 + 9 = 50
	const vcTotal = 13 + 17 + 11 + 9
	// Offset of wTotalLength field in VC header: after 9 (config) + 8 (IAD)
	// + 9 (VC iface) + 3 (bLength+type+subtype) + 2 (bcdUVC) = 31.
	uvcConfigDesc[9+8+9+3+2] = byte(vcTotal & 0xFF)
	uvcConfigDesc[9+8+9+3+2+1] = byte(vcTotal >> 8)

	// VS wTotalLength = VS input header + VS format + VS frame + color
	// = 14 + 27 + 30 + 6 = 77.
	const vsTotal = 14 + 27 + 30 + 6
	// Offset of wTotalLength in VS input header: after 9+8+9+13+17+11+9+9
	// = 85 (config..VS iface alt 0) + 3 (bLength+type+subtype) + 1
	// (bNumFormats) = 89.
	uvcConfigDesc[85+3+1] = byte(vsTotal & 0xFF)
	uvcConfigDesc[85+3+1+1] = byte(vsTotal >> 8)
}

// ---------------------------------------------------------------------------
// Streaming state + pump.
// ---------------------------------------------------------------------------

var (
	uvcStreaming  bool
	uvcFrameIndex uint8  // FID bit toggles per frame; 0 or 1
	uvcFrameOff   uint32 // offset into current frame (bytes, 0..uvcFrameBytes)
	uvcTickCount  uint32 // used to shift the test pattern between frames
	uvcPacket     [uvcIsoMPS]byte

	// Diagnostic counters. UVCStats() dumps them.
	uvcNReqs         uint32
	uvcNGetInfo      uint32
	uvcNGetLen       uint32
	uvcNGetCur       uint32
	uvcNGetMinMax    uint32
	uvcNGetDef       uint32
	uvcNSetCurProbe  uint32
	uvcNSetCurCommit uint32
	uvcNSetInterface uint32
	uvcNSetCurOk     uint32 // SET_CUR finished and SendZlp was reached
	uvcNSetCurTimeo  uint32 // uvcRecvProbe timed out
	uvcNPolls        uint32 // PollUVC entries (after streaming==true gate)
	uvcNSendsEntered uint32 // uvcSendPacket entries
	uvcNSendsExited  uint32 // uvcSendPacket completions (incremented at end)
	uvcLastBReq      uint8
	uvcLastWValueH   uint8
	uvcLastWLength   uint16
)

// UVCStats prints counters for all UVC class requests seen since boot.
// Useful for diagnosing host/device probe-commit negotiation.
func UVCStats() {
	println("uvc: reqs=", uvcNReqs,
		" getInfo=", uvcNGetInfo, " getLen=", uvcNGetLen, " getCur=", uvcNGetCur,
		" getMinMax=", uvcNGetMinMax, " getDef=", uvcNGetDef,
		" setCurProbe=", uvcNSetCurProbe, " setCurCommit=", uvcNSetCurCommit,
		" setIface=", uvcNSetInterface,
		" setCurOk=", uvcNSetCurOk, " setCurTimeo=", uvcNSetCurTimeo)
	println("uvc: pumps polls=", uvcNPolls,
		" sendsIn=", uvcNSendsEntered, " sendsOut=", uvcNSendsExited)
	println("uvc: last bReq=", uvcLastBReq, " cs=", uvcLastWValueH, " wLen=", uvcLastWLength)
	println("uvc: streaming=", uvcStreaming, " frameOff=", uvcFrameOff, " tick=", uvcTickCount)
}

// EnableUVC configures the N6 USB stack as a single-format UVC webcam
// (160×120 YUY2 @ 15 fps). Call BEFORE USBDev.Configure() and INSTEAD of
// usbcdc.EnableUSBCDC(). Order matters: USBDev.Configure() releases the
// USB bus and the host enumerates immediately, so the VID/PID + descriptor
// pointers must be in their final state by then.
func EnableUVC() {
	uvcPatchLengths()

	// Override the board-default PID (0x5740 == CDC ACM) with our UVC PID.
	// sendDescriptor() patches usbDescriptor.Device with usb_VID/usb_PID
	// on every GET_DESCRIPTOR(DEVICE), so just rewriting the bytes in
	// uvcDeviceDesc isn't enough.
	usb_PID = 0x5741

	usbDescriptor.Device = uvcDeviceDesc[:]
	usbDescriptor.Configuration = uvcConfigDesc
	usbDescriptor.HID = nil

	// Wipe CDC's EP assignments — UVC uses only EP0 (control) + EP1 IN
	// (iso). Everything else stays disabled.
	endPoints[usb.CONTROL_ENDPOINT] = usb.ENDPOINT_TYPE_CONTROL
	endPoints[uvcIsoEP] = usb.ENDPOINT_TYPE_ISOCHRONOUS | usb.EndpointIn
	for i := uvcIsoEP + 1; i < len(endPoints); i++ {
		endPoints[i] = usb.ENDPOINT_TYPE_DISABLE
	}
	for i := 0; i < len(usbTxHandler); i++ {
		usbTxHandler[i] = nil
		usbRxHandler[i] = nil
	}

	// Register the iso EP1 IN tx handler — fires from handleIEPInt on
	// XFRC and queues the next packet. Driving the iso refill from the
	// XFRC IRQ instead of polling DTXFSTS is necessary on this core:
	// DTXFSTS only releases 1 word per iso transfer, not the full
	// MPS-worth, so a "wait until DTXFSTS >= MPS" scheme deadlocks
	// after the first packet.
	usbTxHandler[uvcIsoEP] = uvcOnEP1XFRC

	// Register VS interface class-request handler.
	usbSetupHandler[uvcVSInterface] = uvcVSSetup
	usbSetupHandler[uvcVCInterface] = nil
}

// uvcOnEP1XFRC is invoked in IRQ context from handleIEPInt whenever the
// iso IN endpoint completes a transfer. Refills the next packet
// immediately so the EP stays armed across consecutive microframes.
func uvcOnEP1XFRC() {
	if !uvcStreaming {
		return
	}
	uvcSendPacket()
}

// PollUVC is now a no-op safety wrapper. The iso refill path is driven
// by the XFRC IRQ via uvcOnEP1XFRC; calling this from the main loop
// merely enters the streaming/EPENA gate and returns. Kept for backward
// compatibility with mains that still call it.
func PollUVC() {
	if !uvcStreaming {
		return
	}
	uvcNPolls++
	// If EP is already armed, IRQ path will refill on completion.
	if diepctlReg(uvcIsoEP).Get()&otgEPCTL_EPENA != 0 {
		return
	}
	// EP not armed and streaming on — kick a packet (handles the very
	// first send too, though uvcSetInterface already primes).
	uvcSendPacket()
}

// uvcSendPacket assembles one iso packet (2-byte UVC header + payload)
// and pushes it to EP1's TX FIFO.
func uvcSendPacket() {
	uvcNSendsEntered++

	// How many payload bytes this packet carries.
	remaining := uvcFrameBytes - int(uvcFrameOff)
	n := uvcPayloadBytes
	if remaining < n {
		n = remaining
	}

	// UVC payload header (2 bytes). bit 0 = FID (per-frame toggle),
	// bit 1 = EOF, bit 7 = EOH (end-of-header, always 1 for us).
	uvcPacket[0] = 2
	info := byte(0x80) | uvcFrameIndex
	if int(uvcFrameOff)+n >= uvcFrameBytes {
		info |= 0x02 // EOF — last packet of the frame
	}
	uvcPacket[1] = info

	// Payload — generated on the fly (no frame buffer allocation).
	for i := 0; i < n; i++ {
		uvcPacket[2+i] = uvcPixel(uint32(uvcFrameOff) + uint32(i))
	}

	totalLen := uint32(n + 2)

	// Per RM0486 73.15.6 "IN data transfers / Packet write":
	//   1. Program DIEPTSIZ (transfer size + packet count + multi-count)
	//   2. RMW DIEPCTL to set EPENA + CNAK + iso frame-parity hint
	//      (preserving MPSIZ, EPTYP, TXFNUM, USBAEP from EP activation)
	//   3. Write the complete packet's data to the TX FIFO
	// Step 2 must happen BEFORE step 3 — that's the RM-prescribed order.
	dieptsizReg(uvcIsoEP).Set(
		(1 << otgDIEPTSIZ_PKTCNT_Pos) | // PKTCNT=1
			(1 << 29) | // MCNT=01 (1 packet/µframe)
			totalLen, // XFRSIZ
	)

	// Iso parity: program EONUM (bit 16, read-only) via SODDFRM (bit 29)
	// or SEVNFRM (bit 28, named SD0PID in the SVD). Target the upcoming
	// microframe's parity = opposite of current FNSOF[0] in DSTS bit 8.
	even := otg.DSTS.Get()&(1<<8) == 0
	var parityBit uint32
	if even {
		parityBit = otgEPCTL_SODDFRM // upcoming µf is odd
	} else {
		parityBit = otgEPCTL_SD0PID // = SEVNFRM, upcoming µf is even
	}
	diepctlReg(uvcIsoEP).SetBits(otgEPCTL_EPENA | otgEPCTL_CNAK | parityBit)

	// Now push the complete packet's bytes into the TX FIFO. Per RM,
	// the full payload must be present BEFORE the IN token arrives —
	// otherwise the core sends a 0-byte iso IN.
	fifoWrite(uvcIsoEP, uvcPacket[:totalLen])

	// Advance frame position; on frame boundary flip FID.
	uvcFrameOff += uint32(n)
	if uvcFrameOff >= uvcFrameBytes {
		uvcFrameOff = 0
		uvcFrameIndex ^= 1
		uvcTickCount++
	}
	uvcNSendsExited++
}

// uvcPixel returns one byte of the YUY2 test pattern at the given byte
// offset in the frame. YUY2 stores pairs of pixels as Y0 U Y1 V.
// Horizontal gradient with per-frame shift — visually confirms streaming.
func uvcPixel(offset uint32) byte {
	// Reconstruct (x, y) from byte offset. Frame stride = width * 2.
	stride := uint32(uvcWidth * uvcBPP)
	x := (offset % stride) / 2 // pixel x
	compo := offset % 4        // 0=Y0 1=U 2=Y1 3=V
	switch compo {
	case 0, 2:
		// Y channel — horizontal ramp, shifted by frame count.
		return byte(x + uvcTickCount*4)
	default:
		return 128 // U/V at neutral (greyscale look)
	}
}

// ---------------------------------------------------------------------------
// UVC class-request handling (VS interface).
// ---------------------------------------------------------------------------

// uvcVSSetup handles class-specific requests on the VS interface:
//
//	GET_INFO, GET_CUR / SET_CUR on VS_PROBE_CONTROL and VS_COMMIT_CONTROL.
//
// wValueH selects which control; wValueL is 0 for interface-scope.
func uvcVSSetup(setup usb.Setup) bool {
	uvcNReqs++
	uvcLastBReq = setup.BRequest
	uvcLastWValueH = setup.WValueH
	uvcLastWLength = setup.WLength

	// Only interface-scope class requests on the VS interface reach us.
	cs := setup.WValueH
	if cs != uvcVS_PROBE_CONTROL && cs != uvcVS_COMMIT_CONTROL {
		return false // stall unknown control
	}

	switch setup.BRequest {
	case uvcGET_INFO:
		uvcNGetInfo++
		// Single byte: bit 0 = GET supported, bit 1 = SET supported.
		sendDescriptorData([]byte{0x03}, setup.WLength)
		return true

	case uvcGET_LEN:
		uvcNGetLen++
		// Per UVC 1.1 §4.2.1.1: GET_LEN returns the wLength of the
		// associated control as a 16-bit little-endian value. Probe and
		// commit structs are both 34 bytes for UVC 1.1.
		sendDescriptorData([]byte{byte(len(uvcProbe)), 0}, setup.WLength)
		return true

	case uvcGET_DEF:
		uvcNGetDef++
		sendDescriptorData(uvcProbe[:], setup.WLength)
		return true

	case uvcGET_MIN, uvcGET_MAX:
		uvcNGetMinMax++
		sendDescriptorData(uvcProbe[:], setup.WLength)
		return true

	case uvcGET_CUR:
		uvcNGetCur++
		sendDescriptorData(uvcProbe[:], setup.WLength)
		return true

	case uvcSET_CUR:
		if cs == uvcVS_PROBE_CONTROL {
			uvcNSetCurProbe++
		} else {
			uvcNSetCurCommit++
		}
		// Host is writing the probe/commit struct. Accept it (we ignore
		// the content — only one format is supported anyway) by receiving
		// the data phase and ACKing.
		_, err := uvcRecvProbe(int(setup.WLength))
		if err != nil {
			uvcNSetCurTimeo++
			return false
		}
		SendZlp()
		uvcNSetCurOk++
		return true
	}
	return false
}

// uvcRecvProbe consumes a probe/commit data-OUT payload on EP0. The
// control-packet-receive primitive is synchronous (drains RXFLVL from
// IRQ-or-poll context) just like ReceiveUSBControlPacket does for CDC.
func uvcRecvProbe(length int) ([]byte, error) {
	if length > len(uvcProbe) {
		length = len(uvcProbe)
	}
	otg.DOEPTSIZ0.Set(
		(3 << otgDOEPTSIZ0_STUPCNT_Pos) |
			otgDOEPTSIZ0_PKTCNT |
			uint32(length),
	)
	doepctlReg(0).SetBits(otgEPCTL_EPENA | otgEPCTL_CNAK)

	// USB HS SETUP→DATA latency is microseconds; we need a tight timeout
	// here because we busy-wait inside the polling loop (no other
	// PollUSB calls run while we're waiting). 300K iter × ~10 cycles ≈
	// 5 ms — more than enough for a real OUT data packet; if it doesn't
	// arrive, we want to return and let the next PollUSB pass continue
	// servicing other USB events.
	timeout := 300_000
	for epRxCount[0] < uint16(length) {
		if otg.GINTSTS.Get()&otgGINTSTS_RXFLVL != 0 {
			drainRxFifo()
		}
		timeout--
		if timeout == 0 {
			return nil, ErrUSBReadTimeout
		}
	}
	n := epRxCount[0]
	buf := make([]byte, n)
	copy(buf, epRxBuffer[0][:n])
	epRxCount[0] = 0
	return buf, nil
}

// uvcSetInterface is called by the main USB dispatch when a
// SET_INTERFACE targeting the VS interface arrives. Alt 0 = idle,
// alt 1 = streaming.
func uvcSetInterface(ifaceNum uint16, alt uint8) {
	if ifaceNum != uvcVSInterface {
		return
	}
	uvcNSetInterface++

	uvcStreaming = false
	uvcFrameOff = 0
	uvcFrameIndex = 0

	if alt == 0 {
		return
	}
	// alt == 1 — start streaming. PollUVC (and/or the EP1 XFRC handler)
	// will arm and refill the iso EP from here on.
	uvcStreaming = true
}
