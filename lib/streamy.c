#include <freertos/FreeRTOS.h>
#include <freertos/queue.h>
#include <freertos/task.h>

#include <driver/i2s.h>
#include <naos.h>
#include <string.h>

#include "streamy.h"

enum {
  STREAMY_COMMAND_WRITE,
  STREAMY_COMMAND_STOP,
};

typedef struct {
  uint8_t type;
  uint8_t* chunk;
  size_t length;
} streamy_command_t;

static streamy_config_t streamy_config;
static int streamy_chunk_size;
static QueueHandle_t streamy_queue;

static void streamy_task() {
  // prepare command
  streamy_command_t cmd;

  for (;;) {
    // await next command
    xQueueReceive(streamy_queue, &cmd, portMAX_DELAY);

    // handle stop right away
    if (cmd.type == STREAMY_COMMAND_STOP) {
      ESP_ERROR_CHECK(i2s_zero_dma_buffer(I2S_NUM_0))
      continue;
    }

    // assert play command
    if (cmd.type != STREAMY_COMMAND_WRITE) {
      ESP_ERROR_CHECK(ESP_FAIL)
    }

    // write chunk
    size_t bytes_written = 0;
    ESP_ERROR_CHECK(i2s_write(I2S_NUM_0, cmd.chunk, cmd.length, &bytes_written, portMAX_DELAY))

    // free chunk
    free(cmd.chunk);
  }
}

void streamy_monitor() {
  for (;;) {
    // sleep
    naos_delay(streamy_config.update_rate);

    // publish length
    if (naos_status() == NAOS_NETWORKED) {
      naos_publish_l("streamy/queue", (int32_t)uxQueueMessagesWaiting(streamy_queue), 0, false, NAOS_LOCAL);
    }
  }
}

void streamy_init(streamy_config_t config) {
  // store config
  streamy_config = config;

  // create queue
  streamy_queue = xQueueCreate(config.queue_length, sizeof(streamy_command_t));

  // compute chunk size
  streamy_chunk_size = config.sample_rate * config.dma_chunk_length * (config.bit_rate / 8) / 1000;

  // configure driver
  i2s_config_t i2s_config = {
      .mode = I2S_MODE_MASTER | I2S_MODE_TX,
      .sample_rate = config.sample_rate,
      .bits_per_sample = config.bit_rate,
      .channel_format = I2S_CHANNEL_FMT_ONLY_LEFT,
      .communication_format = I2S_COMM_FORMAT_I2S | I2S_COMM_FORMAT_I2S_MSB,
      .intr_alloc_flags = 0,
      .dma_buf_count = config.dma_chunk_num,
      .dma_buf_len = streamy_chunk_size,
      .use_apll = false,
  };
  ESP_ERROR_CHECK(i2s_driver_install(I2S_NUM_0, &i2s_config, 0, NULL))

  // configure pins
  i2s_pin_config_t pin_config = {
      .bck_io_num = config.pin_clk,
      .ws_io_num = config.pin_lrc,
      .data_out_num = config.pin_data,
      .data_in_num = I2S_PIN_NO_CHANGE,
  };
  ESP_ERROR_CHECK(i2s_set_pin(I2S_NUM_0, &pin_config))

  // zero buffer
  ESP_ERROR_CHECK(i2s_zero_dma_buffer(I2S_NUM_0))

  // run task
  xTaskCreatePinnedToCore(streamy_task, "streamy-t", 2048, NULL, 2, NULL, 1);
  xTaskCreatePinnedToCore(streamy_monitor, "streamy-m", 2048, NULL, 3, NULL, 1);
}

void streamy_setup() {
  // subscribe topics
  naos_subscribe("streamy/write", 0, NAOS_LOCAL);
  naos_subscribe("streamy/stop", 0, NAOS_LOCAL);
}

void streamy_handle(const char* topic, uint8_t* payload, size_t len, naos_scope_t scope) {
  // handle write
  if (scope == NAOS_LOCAL && strcmp(topic, "streamy/write") == 0) {
    streamy_write(payload, len);
  }

  // handle stop
  if (scope == NAOS_LOCAL && strcmp(topic, "streamy/stop") == 0) {
    streamy_stop();
  }
}

void streamy_write(uint8_t* data, size_t length) {
  // copy chunk
  uint8_t* chunk = malloc(length);
  memcpy(chunk, data, length);

  // prepare command
  streamy_command_t cmd = {
      .type = STREAMY_COMMAND_WRITE,
      .chunk = chunk,
      .length = length,
  };

  // send command
  if (!xQueueSend(streamy_queue, &cmd, portMAX_DELAY)) {
    naos_log("streamy: failed to queue play command");
  }
}

void streamy_stop() {
  // prepare command
  streamy_command_t cmd = {
      .type = STREAMY_COMMAND_STOP,
  };

  // send command
  if (!xQueueSend(streamy_queue, &cmd, portMAX_DELAY)) {
    naos_log("streamy: failed to queue stop command");
  }
}
