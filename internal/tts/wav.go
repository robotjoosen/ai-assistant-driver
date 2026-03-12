package tts

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
)

const (
	OutputSampleRate = 16000
	OutputChannels   = 1
	OutputBitDepth   = 16
)

func EncodeWAV(pcmData []byte, inputSampleRate, inputBitDepth, inputChannels int) ([]byte, error) {
	if len(pcmData) == 0 {
		return nil, fmt.Errorf("no PCM data provided")
	}

	slog.Debug("WAV encoding input",
		"pcm_size", len(pcmData),
		"sample_rate", inputSampleRate,
		"bit_depth", inputBitDepth,
		"channels", inputChannels,
	)

	resampledData, err := resampleAndConvert(pcmData, inputSampleRate, inputBitDepth, inputChannels)
	if err != nil {
		return nil, fmt.Errorf("failed to process audio: %w", err)
	}

	slog.Debug("WAV encoding after conversion",
		"output_size", len(resampledData),
		"output_sample_rate", OutputSampleRate,
		"output_bit_depth", OutputBitDepth,
		"output_channels", OutputChannels,
	)

	header := createWAVHeader(len(resampledData), OutputSampleRate, OutputBitDepth, OutputChannels)

	slog.Debug("WAV header",
		"header_size", len(header),
		"riff_magic", string(header[0:4]),
		"wave_magic", string(header[8:12]),
		"data_magic", string(header[36:40]),
	)

	var wavData bytes.Buffer
	wavData.Write(header)
	wavData.Write(resampledData)

	slog.Debug("WAV encoding complete", "total_size", wavData.Len())

	return wavData.Bytes(), nil
}

func resampleAndConvert(pcmData []byte, sampleRate, bitDepth, channels int) ([]byte, error) {
	if sampleRate == OutputSampleRate && bitDepth == OutputBitDepth && channels == OutputChannels {
		return pcmData, nil
	}

	var converted []byte

	bytesPerSample := bitDepth / 8

	if bitDepth == 16 && channels == 1 {
		converted = pcmData
	} else if bitDepth == 16 && channels > 1 {
		numSamples := len(pcmData) / (bytesPerSample * channels)
		converted = make([]byte, numSamples*2)
		for i := 0; i < numSamples; i++ {
			offset := i * bytesPerSample * channels
			left := pcmData[offset : offset+2]
			converted[i*2] = left[0]
			converted[i*2+1] = left[1]
		}
	} else if bitDepth == 32 && channels == 1 {
		numSamples := len(pcmData) / 4
		converted = make([]byte, numSamples*2)
		for i := 0; i < numSamples; i++ {
			offset := i * 4
			val := int32(pcmData[offset]) | int32(pcmData[offset+1])<<8 | int32(pcmData[offset+2])<<16 | int32(pcmData[offset+3])<<24
			sampled16 := int16(val >> 16)
			binary.LittleEndian.PutUint16(converted[i*2:], uint16(sampled16))
		}
	} else if bitDepth == 32 && channels > 1 {
		numSamples := len(pcmData) / (4 * channels)
		converted = make([]byte, numSamples*2)
		for i := 0; i < numSamples; i++ {
			offset := i * 4 * channels
			left := int32(pcmData[offset]) | int32(pcmData[offset+1])<<8 | int32(pcmData[offset+2])<<16 | int32(pcmData[offset+3])<<24
			sampled16 := int16(left >> 16)
			binary.LittleEndian.PutUint16(converted[i*2:], uint16(sampled16))
		}
	} else {
		converted = pcmData
	}

	if sampleRate != OutputSampleRate {
		converted = linearResample(converted, sampleRate, OutputSampleRate)
	}

	return converted, nil
}

func linearResample(data []byte, srcRate, dstRate int) []byte {
	if srcRate == dstRate {
		return data
	}

	ratio := float64(dstRate) / float64(srcRate)
	numSamples := len(data) / 2
	newNumSamples := int(float64(numSamples) * ratio)

	resampled := make([]byte, newNumSamples*2)
	for i := 0; i < newNumSamples; i++ {
		srcIdx := float64(i) / ratio
		srcIdxInt := int(srcIdx)
		frac := srcIdx - float64(srcIdxInt)

		if srcIdxInt+1 >= numSamples {
			copy(resampled[i*2:], data[srcIdxInt*2:])
			break
		}

		sample1 := int16(binary.LittleEndian.Uint16(data[srcIdxInt*2:]))
		sample2 := int16(binary.LittleEndian.Uint16(data[(srcIdxInt+1)*2:]))

		sample := int16(float64(sample1)*(1-frac) + float64(sample2)*frac)
		binary.LittleEndian.PutUint16(resampled[i*2:], uint16(sample))
	}

	return resampled
}

func createWAVHeader(dataSize int, sampleRate, bitsPerSample, channels int) []byte {
	buf := make([]byte, 44)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+dataSize))
	copy(buf[8:12], "WAVE")

	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sampleRate*channels*bitsPerSample/8))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(channels*bitsPerSample/8))
	binary.LittleEndian.PutUint16(buf[34:36], uint16(bitsPerSample))

	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))

	return buf
}

func SaveWAVFile(pcmData []byte, sampleRate, bitDepth, channels int) (string, error) {
	wavData, err := EncodeWAV(pcmData, sampleRate, bitDepth, channels)
	if err != nil {
		return "", fmt.Errorf("failed to encode WAV: %w", err)
	}

	dir := "./data/tts"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create tts directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "tts-*.wav")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(wavData); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write WAV file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	return tmpFile.Name(), nil
}
