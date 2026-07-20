package com.claudephone

import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.Bundle
import android.speech.RecognitionListener
import android.speech.RecognizerIntent
import android.speech.SpeechRecognizer
import java.util.Locale

enum class OnDeviceSpeechAvailability { Available, UnsupportedVersion, Unavailable }

fun onDeviceSpeechAvailability(sdkInt: Int, isAvailable: () -> Boolean): OnDeviceSpeechAvailability = when {
    sdkInt < Build.VERSION_CODES.S -> OnDeviceSpeechAvailability.UnsupportedVersion
    !isAvailable() -> OnDeviceSpeechAvailability.Unavailable
    else -> OnDeviceSpeechAvailability.Available
}

fun onDeviceSpeechErrorMessage(code: Int): String = when (code) {
    SpeechRecognizer.ERROR_INSUFFICIENT_PERMISSIONS -> "没有麦克风权限，请在系统设置中允许访问"
    SpeechRecognizer.ERROR_NO_MATCH, SpeechRecognizer.ERROR_SPEECH_TIMEOUT -> "没有听清，请重试"
    SpeechRecognizer.ERROR_RECOGNIZER_BUSY -> "端侧语音识别正忙，请稍后重试"
    SpeechRecognizer.ERROR_LANGUAGE_NOT_SUPPORTED -> "当前语言不支持离线语音输入"
    SpeechRecognizer.ERROR_LANGUAGE_UNAVAILABLE -> "当前语言的离线语音模型不可用"
    SpeechRecognizer.ERROR_NETWORK, SpeechRecognizer.ERROR_NETWORK_TIMEOUT -> "端侧识别失败，未使用网络识别"
    else -> "离线语音识别失败，请重试"
}

sealed interface VoiceState {
    data object Idle : VoiceState
    data object Listening : VoiceState
    data object Processing : VoiceState
    data class Unavailable(val message: String) : VoiceState
    data class Failed(val message: String) : VoiceState
}

interface VoiceCallbacks {
    fun onText(text: String, final: Boolean)
    fun onState(state: VoiceState)
}

class OnDeviceSpeechController(
    private val context: Context,
    private val callbacks: VoiceCallbacks,
) : RecognitionListener {
    private var recognizer: SpeechRecognizer? = null

    fun toggle(locale: Locale = Locale.getDefault()) {
        if (recognizer != null) {
            stop()
        } else {
            start(locale)
        }
    }

    private fun start(locale: Locale) {
        when (onDeviceSpeechAvailability(Build.VERSION.SDK_INT) {
            SpeechRecognizer.isOnDeviceRecognitionAvailable(context)
        }) {
            OnDeviceSpeechAvailability.UnsupportedVersion -> {
                callbacks.onState(VoiceState.Unavailable("Android 12 及以上才支持离线语音输入"))
                return
            }
            OnDeviceSpeechAvailability.Unavailable -> {
                callbacks.onState(VoiceState.Unavailable("当前设备或语言不支持离线语音输入"))
                return
            }
            OnDeviceSpeechAvailability.Available -> Unit
        }
        val active = SpeechRecognizer.createOnDeviceSpeechRecognizer(context)
        recognizer = active
        active.setRecognitionListener(this)
        active.startListening(Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).apply {
            putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
            putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS, true)
            putExtra(RecognizerIntent.EXTRA_LANGUAGE, locale.toLanguageTag())
        })
        callbacks.onState(VoiceState.Listening)
    }

    fun stop() {
        recognizer?.stopListening()
        callbacks.onState(VoiceState.Processing)
    }

    fun destroy() {
        recognizer?.cancel()
        recognizer?.destroy()
        recognizer = null
        callbacks.onState(VoiceState.Idle)
    }

    override fun onReadyForSpeech(params: Bundle?) = Unit
    override fun onBeginningOfSpeech() = Unit
    override fun onRmsChanged(rmsdB: Float) = Unit
    override fun onBufferReceived(buffer: ByteArray?) = Unit
    override fun onEndOfSpeech() { callbacks.onState(VoiceState.Processing) }
    override fun onEvent(eventType: Int, params: Bundle?) = Unit

    override fun onPartialResults(partialResults: Bundle?) {
        bestText(partialResults)?.let { callbacks.onText(it, false) }
    }

    override fun onResults(results: Bundle?) {
        bestText(results)?.let { callbacks.onText(it, true) }
        destroy()
    }

    override fun onError(error: Int) {
        recognizer?.destroy()
        recognizer = null
        callbacks.onState(VoiceState.Failed(onDeviceSpeechErrorMessage(error)))
    }

    private fun bestText(bundle: Bundle?): String? =
        bundle?.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION)
            ?.firstOrNull()
            ?.trim()
            ?.takeIf { it.isNotEmpty() }
}
