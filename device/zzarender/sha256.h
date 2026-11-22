// Minimal SHA-256 (FIPS 180-4), no dependencies.
#pragma once
#include <cstddef>
#include <cstdint>

struct ZzaSha256 {
  uint32_t h[8];
  uint8_t buf[64];
  uint64_t total;  // bytes processed
  size_t fill;     // bytes in buf

  ZzaSha256();
  void update(const uint8_t* data, size_t len);
  void final(uint8_t out[32]);

 private:
  void block(const uint8_t* p);
};
