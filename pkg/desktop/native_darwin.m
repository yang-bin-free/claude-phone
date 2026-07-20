#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#import "native_darwin.h"
#include <string.h>

@interface CPWindowDelegate : NSObject <NSWindowDelegate>
@end

@implementation CPWindowDelegate
- (BOOL)windowShouldClose:(NSWindow *)sender {
    [sender orderOut:nil];
    return NO;
}
@end

static char cpWindowDelegateKey;
static char cpReopenWindowKey;

static BOOL cpApplicationShouldHandleReopen(id self, SEL _cmd, NSApplication *sender, BOOL hasVisibleWindows) {
    NSWindow *window = objc_getAssociatedObject(self, &cpReopenWindowKey);
    if (window != nil) {
        cpShowWindow((__bridge void *)window);
    }
    return YES;
}

void cpConfigureWindow(void *windowPtr) {
    NSWindow *window = (__bridge NSWindow *)windowPtr;
    CPWindowDelegate *delegate = [CPWindowDelegate new];
    window.delegate = delegate;
    window.releasedWhenClosed = NO;
    objc_setAssociatedObject(window, &cpWindowDelegateKey, delegate, OBJC_ASSOCIATION_RETAIN_NONATOMIC);

    id appDelegate = NSApp.delegate;
    if (appDelegate != nil) {
        objc_setAssociatedObject(appDelegate, &cpReopenWindowKey, window, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
        class_addMethod(object_getClass(appDelegate),
                        @selector(applicationShouldHandleReopen:hasVisibleWindows:),
                        (IMP)cpApplicationShouldHandleReopen,
                        "c@:@c");
    }
}

void cpShowWindow(void *windowPtr) {
    NSWindow *window = (__bridge NSWindow *)windowPtr;
    [NSApp activateIgnoringOtherApps:YES];
    [window makeKeyAndOrderFront:nil];
}

void cpHideWindow(void *windowPtr) {
    NSWindow *window = (__bridge NSWindow *)windowPtr;
    [window orderOut:nil];
}

static char *cpChooseDirectoryOnMainThread(void) {
    NSWindow *previousWindow = NSApp.keyWindow ?: NSApp.mainWindow;
    [NSApp unhide:nil];
    [NSApp activateIgnoringOtherApps:YES];
    if (previousWindow != nil) {
        [previousWindow makeKeyAndOrderFront:nil];
    }

    NSOpenPanel *panel = [NSOpenPanel openPanel];
    panel.canChooseDirectories = YES;
    panel.canChooseFiles = NO;
    panel.allowsMultipleSelection = NO;
    panel.canCreateDirectories = NO;
    panel.prompt = @"选择";
    NSModalResponse response = [panel runModal];

    [NSApp unhide:nil];
    [NSApp activateIgnoringOtherApps:YES];
    if (previousWindow != nil) {
        [previousWindow makeKeyAndOrderFront:nil];
    }
    if (response != NSModalResponseOK || panel.URL == nil) {
        return strdup("");
    }
    return strdup(panel.URL.path.UTF8String);
}

char *caChooseDirectory(void) {
    if ([NSThread isMainThread]) {
        return cpChooseDirectoryOnMainThread();
    }
    __block char *result = NULL;
    dispatch_sync(dispatch_get_main_queue(), ^{
        result = cpChooseDirectoryOnMainThread();
    });
    return result;
}

static NSPasteboard *cpPasteboard(const char *name) {
    if (name == NULL || name[0] == '\0') {
        return [NSPasteboard generalPasteboard];
    }
    NSString *pasteboardName = [NSString stringWithUTF8String:name];
    return [NSPasteboard pasteboardWithName:pasteboardName];
}

int caCopyTextToPasteboard(const char *text, const char *name) {
    @autoreleasepool {
        NSString *value = text == NULL ? @"" : [NSString stringWithUTF8String:text];
        if (value == nil) {
            return 0;
        }
        NSPasteboard *pasteboard = cpPasteboard(name);
        [pasteboard clearContents];
        return [pasteboard setString:value forType:NSPasteboardTypeString] ? 1 : 0;
    }
}

char *caReadTextFromPasteboard(const char *name) {
    @autoreleasepool {
        NSString *value = [cpPasteboard(name) stringForType:NSPasteboardTypeString];
        return value == nil ? NULL : strdup(value.UTF8String);
    }
}

void caReleasePasteboard(const char *name) {
    @autoreleasepool {
        if (name == NULL || name[0] == '\0') {
            return;
        }
        [cpPasteboard(name) releaseGlobally];
    }
}
