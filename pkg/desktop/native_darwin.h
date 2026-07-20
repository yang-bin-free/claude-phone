#ifndef CLAUDE_PHONE_NATIVE_DARWIN_H
#define CLAUDE_PHONE_NATIVE_DARWIN_H

void cpConfigureWindow(void *window);
void cpShowWindow(void *window);
void cpHideWindow(void *window);
char *caChooseDirectory(void);
int caCopyTextToPasteboard(const char *text, const char *name);
char *caReadTextFromPasteboard(const char *name);
void caReleasePasteboard(const char *name);

#endif
