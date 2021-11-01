#include <naos.h>
#include <string.h>

#include <streamy.h>

static void online() {
  // setup
  streamy_setup();
}

static void message(const char *topic, uint8_t *payload, size_t len, naos_scope_t scope) {
  // handle
  streamy_handle(topic, payload, len, scope);
}

static naos_param_t params[] = {};

static naos_config_t config = {
    .device_type = "streamy",
    .firmware_version = "0.1.0",
    .parameters = params,
    .num_parameters = sizeof(params) / sizeof(naos_param_t),
    .online_callback = online,
    .message_callback = message,
};

void app_main() {
  // initialize naos
  naos_init(&config);

  // prepare config sound
  streamy_config_t config = {
      .pin_clk = 12,
      .pin_data = 13,
      .pin_lrc = 14,
      .sample_rate = 44100,
      .bit_rate = 16,
      .dma_chunk_length = 10,
      .dma_chunk_num = 3,
      .queue_length = 16,
      .update_rate = 100,
  };

  // initialize
  streamy_init(config);
}
