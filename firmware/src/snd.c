#include <freertos/FreeRTOS.h>
#include <freertos/queue.h>
#include <freertos/task.h>

#include <driver/i2s.h>
#include <naos.h>
#include <string.h>

#include "snd.h"

#define SND_I2S_NUM 0
#define SND_PIN_CLK 12
#define SND_PIN_DATA 13
#define SND_PIN_LRC 14

#define SND_SAMPLE_RATE 44100
#define SND_BIT_RATE 16

#define SND_QUEUE 16
#define SND_BUFFERS 4
#define SND_CHUNK 882  // 44100 / 1s * 10ms * 2 bytes

enum {
  SND_COMMAND_WRITE,
  SND_COMMAND_STOP,
};

typedef struct {
  uint8_t type;
  uint8_t * chunk;
  size_t length;
} snd_command_t;

static QueueHandle_t snd_queue;

static void snd_task(void* _) {
  // prepare command
  snd_command_t cmd;

  for (;;) {
    // await next command
    xQueueReceive(snd_queue, &cmd, portMAX_DELAY);

    // handle stop right away
    if (cmd.type == SND_COMMAND_STOP) {
      i2s_zero_dma_buffer(SND_I2S_NUM);
      continue;
    }

    // assert play command
    if (cmd.type != SND_COMMAND_WRITE) {
      ESP_ERROR_CHECK(ESP_FAIL);
    }

    // write chunk
    size_t bytes_written = 0;
    ESP_ERROR_CHECK(i2s_write(SND_I2S_NUM, cmd.chunk, cmd.length, &bytes_written, portMAX_DELAY));

    // log
    naos_log("queue length: %d", uxQueueMessagesWaiting(snd_queue));

    // free chunk
    free(cmd.chunk);
  }
}

void snd_init() {
  // create queue
  snd_queue = xQueueCreate(SND_QUEUE, sizeof(snd_command_t));

  // configure driver
  static const i2s_config_t i2s_config = {
      .mode = I2S_MODE_MASTER | I2S_MODE_TX,
      .sample_rate = SND_SAMPLE_RATE,
      .bits_per_sample = SND_BIT_RATE,
      .channel_format = I2S_CHANNEL_FMT_ONLY_LEFT,
      .communication_format = I2S_COMM_FORMAT_I2S | I2S_COMM_FORMAT_I2S_MSB,
      .intr_alloc_flags = 0,
      .dma_buf_count = SND_BUFFERS,
      .dma_buf_len = SND_CHUNK,
      .use_apll = false,
  };
  ESP_ERROR_CHECK(i2s_driver_install(SND_I2S_NUM, &i2s_config, 0, NULL));

  // configure pins
  static const i2s_pin_config_t pin_config = {
      .bck_io_num = SND_PIN_CLK,
      .ws_io_num = SND_PIN_LRC,
      .data_out_num = SND_PIN_DATA,
      .data_in_num = I2S_PIN_NO_CHANGE,
  };
  ESP_ERROR_CHECK(i2s_set_pin(SND_I2S_NUM, &pin_config));

  // run task
  xTaskCreatePinnedToCore(snd_task, "snd", 2048, NULL, 2, NULL, 1);
}

void snd_write(uint8_t * data, size_t length) {
  // copy chunk
  uint8_t * chunk = malloc(length);
  memcpy(chunk, data, length);

  // prepare command
  snd_command_t cmd = {
      .type = SND_COMMAND_WRITE,
      .chunk = chunk,
      .length = length,
  };

  // send command
  if (!xQueueSend(snd_queue, &cmd, portMAX_DELAY)) {
    naos_log("snd: failed to queue play command");
  }
}

void snd_stop() {
  // prepare command
  snd_command_t cmd = {
      .type = SND_COMMAND_STOP,
  };

  // send command
  if (!xQueueSend(snd_queue, &cmd, portMAX_DELAY)) {
    naos_log("snd: failed to queue stop command");
  }
}
