package com.claudephone

import android.speech.SpeechRecognizer
import org.junit.Assert.assertEquals
import org.junit.Test

class OnDeviceSpeechPolicyTest {
    @Test
    fun api30IsUnavailableWithoutCheckingRecognizerService() {
        var checks = 0
        val availability = onDeviceSpeechAvailability(30) { checks++; true }
        assertEquals(OnDeviceSpeechAvailability.UnsupportedVersion, availability)
        assertEquals(0, checks)
    }

    @Test
    fun missingOnDeviceServiceIsUnavailable() {
        val availability = onDeviceSpeechAvailability(35) { false }
        assertEquals(OnDeviceSpeechAvailability.Unavailable, availability)
    }

    @Test
    fun api31WithOnDeviceServiceIsAvailable() {
        val availability = onDeviceSpeechAvailability(31) { true }
        assertEquals(OnDeviceSpeechAvailability.Available, availability)
    }

    @Test
    fun networkErrorsNeverSuggestACloudFallback() {
        val message = onDeviceSpeechErrorMessage(SpeechRecognizer.ERROR_NETWORK)
        assertEquals("端侧识别失败，未使用网络识别", message)
    }

    @Test
    fun languageUnavailableHasActionableMessage() {
        val message = onDeviceSpeechErrorMessage(SpeechRecognizer.ERROR_LANGUAGE_UNAVAILABLE)
        assertEquals("当前语言的离线语音模型不可用", message)
    }
}
