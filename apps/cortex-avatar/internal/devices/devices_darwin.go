// Package devices provides native hardware device enumeration for macOS
package devices

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreAudio -framework AVFoundation -framework Foundation

#import <CoreAudio/CoreAudio.h>
#import <AVFoundation/AVFoundation.h>
#import <Foundation/Foundation.h>

typedef struct {
    char* deviceID;
    char* name;
    char* kind;
} DeviceInfo;

typedef struct {
    DeviceInfo* devices;
    int count;
} DeviceList;

// Get audio devices using CoreAudio
DeviceList getAudioDevices() {
    DeviceList result = {NULL, 0};

    AudioObjectPropertyAddress propertyAddress = {
        kAudioHardwarePropertyDevices,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };

    UInt32 dataSize = 0;
    OSStatus status = AudioObjectGetPropertyDataSize(
        kAudioObjectSystemObject,
        &propertyAddress,
        0, NULL,
        &dataSize
    );

    if (status != noErr) {
        return result;
    }

    int deviceCount = dataSize / sizeof(AudioDeviceID);
    if (deviceCount == 0) {
        return result;
    }

    AudioDeviceID* deviceIDs = (AudioDeviceID*)malloc(dataSize);
    status = AudioObjectGetPropertyData(
        kAudioObjectSystemObject,
        &propertyAddress,
        0, NULL,
        &dataSize,
        deviceIDs
    );

    if (status != noErr) {
        free(deviceIDs);
        return result;
    }

    // Allocate for max possible (input + output for each device)
    result.devices = (DeviceInfo*)malloc(sizeof(DeviceInfo) * deviceCount * 2);
    result.count = 0;

    for (int i = 0; i < deviceCount; i++) {
        AudioDeviceID deviceID = deviceIDs[i];

        // Get device name
        CFStringRef deviceName = NULL;
        dataSize = sizeof(deviceName);
        propertyAddress.mSelector = kAudioDevicePropertyDeviceNameCFString;
        propertyAddress.mScope = kAudioObjectPropertyScopeGlobal;

        status = AudioObjectGetPropertyData(
            deviceID,
            &propertyAddress,
            0, NULL,
            &dataSize,
            &deviceName
        );

        if (status != noErr || deviceName == NULL) {
            continue;
        }

        char nameBuffer[256];
        CFStringGetCString(deviceName, nameBuffer, 256, kCFStringEncodingUTF8);
        CFRelease(deviceName);

        // Check for input capability
        propertyAddress.mSelector = kAudioDevicePropertyStreamConfiguration;
        propertyAddress.mScope = kAudioDevicePropertyScopeInput;

        status = AudioObjectGetPropertyDataSize(deviceID, &propertyAddress, 0, NULL, &dataSize);
        if (status == noErr && dataSize > 0) {
            AudioBufferList* bufferList = (AudioBufferList*)malloc(dataSize);
            status = AudioObjectGetPropertyData(deviceID, &propertyAddress, 0, NULL, &dataSize, bufferList);

            if (status == noErr && bufferList->mNumberBuffers > 0) {
                int hasChannels = 0;
                for (UInt32 j = 0; j < bufferList->mNumberBuffers; j++) {
                    if (bufferList->mBuffers[j].mNumberChannels > 0) {
                        hasChannels = 1;
                        break;
                    }
                }
                if (hasChannels) {
                    result.devices[result.count].deviceID = (char*)malloc(32);
                    sprintf(result.devices[result.count].deviceID, "input_%d", deviceID);
                    result.devices[result.count].name = strdup(nameBuffer);
                    result.devices[result.count].kind = strdup("audioinput");
                    result.count++;
                }
            }
            free(bufferList);
        }

        // Check for output capability
        propertyAddress.mScope = kAudioDevicePropertyScopeOutput;

        status = AudioObjectGetPropertyDataSize(deviceID, &propertyAddress, 0, NULL, &dataSize);
        if (status == noErr && dataSize > 0) {
            AudioBufferList* bufferList = (AudioBufferList*)malloc(dataSize);
            status = AudioObjectGetPropertyData(deviceID, &propertyAddress, 0, NULL, &dataSize, bufferList);

            if (status == noErr && bufferList->mNumberBuffers > 0) {
                int hasChannels = 0;
                for (UInt32 j = 0; j < bufferList->mNumberBuffers; j++) {
                    if (bufferList->mBuffers[j].mNumberChannels > 0) {
                        hasChannels = 1;
                        break;
                    }
                }
                if (hasChannels) {
                    result.devices[result.count].deviceID = (char*)malloc(32);
                    sprintf(result.devices[result.count].deviceID, "output_%d", deviceID);
                    result.devices[result.count].name = strdup(nameBuffer);
                    result.devices[result.count].kind = strdup("audiooutput");
                    result.count++;
                }
            }
            free(bufferList);
        }
    }

    free(deviceIDs);
    return result;
}

// Get video devices using AVFoundation
DeviceList getVideoDevices() {
    DeviceList result = {NULL, 0};

    @autoreleasepool {
        NSArray<AVCaptureDevice*>* devices = [AVCaptureDevice devicesWithMediaType:AVMediaTypeVideo];

        if (devices.count == 0) {
            return result;
        }

        result.devices = (DeviceInfo*)malloc(sizeof(DeviceInfo) * devices.count);
        result.count = (int)devices.count;

        for (int i = 0; i < devices.count; i++) {
            AVCaptureDevice* device = devices[i];
            result.devices[i].deviceID = strdup([device.uniqueID UTF8String]);
            result.devices[i].name = strdup([device.localizedName UTF8String]);
            result.devices[i].kind = strdup("videoinput");
        }
    }

    return result;
}

void freeDeviceList(DeviceList list) {
    for (int i = 0; i < list.count; i++) {
        free(list.devices[i].deviceID);
        free(list.devices[i].name);
        free(list.devices[i].kind);
    }
    if (list.devices != NULL) {
        free(list.devices);
    }
}
*/
import "C"
import (
	"unsafe"
)

// Device represents a hardware device
type Device struct {
	DeviceID string `json:"deviceId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // audioinput, audiooutput, videoinput
}

// ListAudioDevices returns all audio input and output devices
func ListAudioDevices() []Device {
	cList := C.getAudioDevices()
	defer C.freeDeviceList(cList)

	devices := make([]Device, 0, int(cList.count))

	if cList.count == 0 || cList.devices == nil {
		return devices
	}

	// Convert C array to Go slice
	cDevices := (*[1 << 20]C.DeviceInfo)(unsafe.Pointer(cList.devices))[:cList.count:cList.count]

	for _, cDev := range cDevices {
		devices = append(devices, Device{
			DeviceID: C.GoString(cDev.deviceID),
			Name:     C.GoString(cDev.name),
			Kind:     C.GoString(cDev.kind),
		})
	}

	return devices
}

// ListVideoDevices returns all video capture devices
func ListVideoDevices() []Device {
	cList := C.getVideoDevices()
	defer C.freeDeviceList(cList)

	devices := make([]Device, 0, int(cList.count))

	if cList.count == 0 || cList.devices == nil {
		return devices
	}

	// Convert C array to Go slice
	cDevices := (*[1 << 20]C.DeviceInfo)(unsafe.Pointer(cList.devices))[:cList.count:cList.count]

	for _, cDev := range cDevices {
		devices = append(devices, Device{
			DeviceID: C.GoString(cDev.deviceID),
			Name:     C.GoString(cDev.name),
			Kind:     C.GoString(cDev.kind),
		})
	}

	return devices
}

// ListAllDevices returns all audio and video devices
func ListAllDevices() []Device {
	audio := ListAudioDevices()
	video := ListVideoDevices()
	return append(audio, video...)
}

// ListMicrophones returns only audio input devices
func ListMicrophones() []Device {
	devices := ListAudioDevices()
	mics := make([]Device, 0)
	for _, d := range devices {
		if d.Kind == "audioinput" {
			mics = append(mics, d)
		}
	}
	return mics
}

// ListSpeakers returns only audio output devices
func ListSpeakers() []Device {
	devices := ListAudioDevices()
	speakers := make([]Device, 0)
	for _, d := range devices {
		if d.Kind == "audiooutput" {
			speakers = append(speakers, d)
		}
	}
	return speakers
}

// ListCameras returns only video input devices
func ListCameras() []Device {
	return ListVideoDevices()
}
