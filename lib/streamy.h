#ifndef STREAMY
#define STREAMY

void streamy_init();
void streamy_write(uint8_t* chunk, size_t length);
void streamy_stop();

#endif
