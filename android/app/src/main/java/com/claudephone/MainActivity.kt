package com.claudephone

import android.app.Activity
import android.os.Bundle
import android.widget.TextView
import androidlib.Androidlib

class MainActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // ★ P0a 核心验证：Kotlin 调用 Go 函数
        val greeting = Androidlib.hello("阿彬")

        val tv = TextView(this)
        tv.text = greeting
        tv.textSize = 24f
        setContentView(tv)
    }
}
