#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#import "native_darwin.h"

@interface CPWindowDelegate : NSObject <NSWindowDelegate>
@end

@implementation CPWindowDelegate
- (BOOL)windowShouldClose:(NSWindow *)sender {
    [sender orderOut:nil];
    return NO;
}
@end

static char cpWindowDelegateKey;

void cpConfigureWindow(void *windowPtr) {
    NSWindow *window = (__bridge NSWindow *)windowPtr;
    CPWindowDelegate *delegate = [CPWindowDelegate new];
    window.delegate = delegate;
    window.releasedWhenClosed = NO;
    objc_setAssociatedObject(window, &cpWindowDelegateKey, delegate, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
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
