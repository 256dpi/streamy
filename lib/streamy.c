#include <freertos/FreeRTOS.h>
#include <freertos/queue.h>
#include <freertos/task.h>

#include <driver/i2s.h>
#include <naos.h>
#include <string.h>

#include "streamy.h"

#define STREAMY_I2S_NUM 0
#define STREAMY_PIN_CLK 12
#define STREAMY_PIN_DATA 13
#define STREAMY_PIN_LRC 14

#define STREAMY_SAMPLE_RATE 44100
#define STREAMY_BIT_RATE 16

#define STREAMY_QUEUE_LENGTH 16
#define STREAMY_DMA_CHUNK 44100 * 10 / 1000 * 2  // 10ms @ 44100Hz/16bit
#define STREAMY_DMA_CHUNKS 3                     // 30ms

#define STREAMY_UPDATE_MS 100

enum {
  STREAMY_COMMAND_WRITE,
  STREAMY_COMMAND_STOP,
};

typedef struct {
  uint8_t type;
  uint8_t* chunk;
  size_t length;
} streamy_command_t;

static QueueHandle_t streamy_queue;

static void streamy_task() {
  // prepare command
  streamy_command_t cmd;

  for (;;) {
    // await next command
    xQueueReceive(streamy_queue, &cmd, portMAX_DELAY);

    // handle stop right away
    if (cmd.type == STREAMY_COMMAND_STOP) {
      ESP_ERROR_CHECK(i2s_zero_dma_buffer(STREAMY_I2S_NUM))
      continue;
    }

    // assert play command
    if (cmd.type != STREAMY_COMMAND_WRITE) {
      ESP_ERROR_CHECK(ESP_FAIL)
    }

    // write chunk
    size_t bytes_written = 0;
    ESP_ERROR_CHECK(i2s_write(STREAMY_I2S_NUM, cmd.chunk, cmd.length, &bytes_written, portMAX_DELAY))

    // free chunk
    free(cmd.chunk);
  }
}

void streamy_monitor() {
  for (;;) {
    // sleep
    naos_delay(STREAMY_UPDATE_MS);

    // publish length
    if (naos_status() == NAOS_NETWORKED) {
      naos_publish_l("queue", (int32_t)uxQueueMessagesWaiting(streamy_queue), 0, false, NAOS_LOCAL);
    }
  }
}

void streamy_init() {
  // create queue
  streamy_queue = xQueueCreate(STREAMY_QUEUE_LENGTH, sizeof(streamy_command_t));

  // configure driver
  static const i2s_config_t i2s_config = {
      .mode = I2S_MODE_MASTER | I2S_MODE_TX,
      .sample_rate = STREAMY_SAMPLE_RATE,
      .bits_per_sample = STREAMY_BIT_RATE,
      .channel_format = I2S_CHANNEL_FMT_ONLY_LEFT,
      .communication_format = I2S_COMM_FORMAT_I2S | I2S_COMM_FORMAT_I2S_MSB,
      .intr_alloc_flags = 0,
      .dma_buf_count = STREAMY_DMA_CHUNKS,
      .dma_buf_len = STREAMY_DMA_CHUNK,
      .use_apll = false,
  };
  ESP_ERROR_CHECK(i2s_driver_install(STREAMY_I2S_NUM, &i2s_config, 0, NULL))

  // configure pins
  static const i2s_pin_config_t pin_config = {
      .bck_io_num = STREAMY_PIN_CLK,
      .ws_io_num = STREAMY_PIN_LRC,
      .data_out_num = STREAMY_PIN_DATA,
      .data_in_num = I2S_PIN_NO_CHANGE,
  };
  ESP_ERROR_CHECK(i2s_set_pin(STREAMY_I2S_NUM, &pin_config))

  // zero buffer
  ESP_ERROR_CHECK(i2s_zero_dma_buffer(STREAMY_I2S_NUM))

  // run task
  xTaskCreatePinnedToCore(streamy_task, "streamy-t", 2048, NULL, 2, NULL, 1);
  xTaskCreatePinnedToCore(streamy_monitor, "streamy-m", 2048, NULL, 3, NULL, 1);
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
    naos_log("snd: failed to queue play command");
  }
}

void streamy_stop() {
  // prepare command
  streamy_command_t cmd = {
      .type = STREAMY_COMMAND_STOP,
  };

  // send command
  if (!xQueueSend(streamy_queue, &cmd, portMAX_DELAY)) {
    naos_log("snd: failed to queue stop command");
  }
}
