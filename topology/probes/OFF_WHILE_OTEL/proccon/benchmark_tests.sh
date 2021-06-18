export NAME=2_server_identifier
go test -v --bench . -run '^$' -test.bench BenchmarkProcessMetricSerial -memprofile benchmark/${NAME}_serial.raw.mem.pprof \
  -cpuprofile benchmark/${NAME}_serial.raw.cpu.pprof \
  -blockprofile benchmark/${NAME}_serie.raw.block.pprof > benchmark/${NAME}_serial.raw && \
  go test -v --bench . -run '^$' -test.bench BenchmarkProcessMetricParallel -memprofile benchmark/${NAME}_paralelo.raw.mem.pprof \
  -cpuprofile benchmark/${NAME}_paralelo.raw.cpu.pprof \
  -blockprofile benchmark/${NAME}_paralelo.raw.block.pprof > benchmark/${NAME}_paralelo.raw
