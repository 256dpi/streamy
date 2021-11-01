#ifndef STREAMY
#define STREAMY

typedef struct {
  int pin_clk;
  int pin_data;
  int pin_lrc;

  int sample_rate;  // 441000
  int bit_rate;     // 16

  int dma_chunk_length;  // 10 (10ms)
  int dma_chunk_num;     // 3 (30ms)

  int queue_length;  // 16
  int update_rate;   // 100 (100ms)
} streamy_config_t;

void streamy_init(streamy_config_t);
void streamy_setup();
void streamy_handle(const char *topic, uint8_t *payload, size_t len, naos_scope_t scope);

void streamy_write(uint8_t *chunk, size_t length);
void streamy_stop();

#endif
