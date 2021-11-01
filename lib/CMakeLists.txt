cmake_minimum_required(VERSION 3.7)
project(streamy)

set(CMAKE_C_STANDARD 99)

include_directories(
        test/build/include
        test/xtensa-esp32-elf/xtensa-esp32-elf/include
        test/esp-idf/components/esp32/include
        test/esp-idf/components/freertos/include
        test/esp-idf/components/log/include
        include/)

set(SOURCE_FILES snd.c snd.h test/src/main.c)

add_library(streamy ${SOURCE_FILES})

add_subdirectory(test/naos)
