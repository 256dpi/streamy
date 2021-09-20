#ifndef SND
#define SND

void snd_init();
void snd_state(bool on);
void snd_write(uint8_t* chunk, size_t length);
void snd_stop();

#endif
