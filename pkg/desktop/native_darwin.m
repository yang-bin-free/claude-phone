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

char *caChooseDirectory(void) {
    NSOpenPanel *panel = [NSOpenPanel openPanel];
    panel.canChooseDirectories = YES;
    panel.canChooseFiles = NO;
    panel.allowsMultipleSelection = NO;
    panel.canCreateDirectories = NO;
    panel.prompt = @"选择";
    if ([panel runModal] != NSModalResponseOK || panel.URL == nil) {
        return strdup("");
    }
    return strdup(panel.URL.path.UTF8String);
}
