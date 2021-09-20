#include <naos.h>
#include <string.h>

#include "snd.h"

static void online() {
  // subscribe topics
  naos_subscribe("write", 0, NAOS_LOCAL);
  naos_subscribe("stop", 0, NAOS_LOCAL);
}

static void message(const char *topic, uint8_t *payload, size_t len, naos_scope_t scope) {
  // handle write
  if (scope == NAOS_LOCAL && strcmp(topic, "write") == 0) {
    snd_write(payload, len);
  }

  // handle stop
  if (scope == NAOS_LOCAL && strcmp(topic, "stop") == 0) {
    snd_stop();
  }
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

  // initialize sound
  snd_init();
}
