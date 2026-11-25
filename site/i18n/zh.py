#!/usr/bin/env python3
"""Builds public/zh/index.html, the Simplified Chinese homepage, from the
English public/index.html: each pair below replaces exact English text.
Run it after editing index.html (python3 site/i18n/zh.py) and commit both;
it fails if an English string is no longer in the page."""
import pathlib, sys

HERE = pathlib.Path(__file__).resolve().parent
SRC = HERE.parent / "public" / "index.html"
OUT = HERE.parent / "public" / "zh" / "index.html"

PAIRS = [
    # head
    ('<html lang="en">', '<html lang="zh-Hans">'),
    ('<title>Mash Piano: let them mash</title>', '<title>Mash Piano：随便按，都好听</title>'),
    ('content="A free piano toy for babies. However they mash the keys, all that comes out is gentle piano music. For Mac, Windows, iPhone, iPad and Android. No ads, no accounts, no data collected."',
     'content="给宝宝的免费钢琴玩具：不管怎么乱按，弹出来的都是轻柔好听的钢琴声。支持 Mac、Windows、iPhone、iPad 和 Android。没有广告、不用注册、不收集任何数据。"'),
    ('<meta property="og:url" content="https://mashpiano.herosizy.dev/">', '<meta property="og:url" content="https://mashpiano.herosizy.dev/zh">'),
    ('<meta property="og:title" content="Mash Piano: let them mash">', '<meta property="og:title" content="Mash Piano：随便按，都好听">'),
    ('content="However they mash the keys, all that comes out is gentle piano music. Free for Mac, Windows, iPhone, iPad and Android."',
     'content="不管怎么乱按，弹出来的都是轻柔好听的钢琴声。Mac、Windows、iPhone、iPad 和 Android 都免费。"'),
    # header
    ('<a class="lang" id="lang" href="/zh" data-lang="zh" lang="zh-Hans" aria-label="中文">中文</a>',
     '<a class="lang" id="lang" href="/" data-lang="en" lang="en" aria-label="English">EN</a>'),
    ('aria-label="Switch between light and dark"', 'aria-label="切换浅色和深色"'),
    ('<a class="get" href="#get">All downloads</a>', '<a class="get" href="#get">全部下载</a>'),
    # hero
    ('<h1 aria-label="Let them mash."><span aria-hidden="true">Let them <span class="word">',
     '<h1 aria-label="随便按，都好听。"><span aria-hidden="true">随便按，<span class="word">'),
    ('<span class="m">m</span><span class="m">a</span><span class="m">s</span><span class="m">h</span><span class="m">.</span>',
     '<span class="m">都</span><span class="m">好</span><span class="m">听</span><span class="m">。</span>'),
    ('<p class="lede">Mash Piano is a piano toy for babies. Every slap, tap and fistful of keys plays a soft piano note, and every note sounds good with the others.</p>',
     '<p class="lede">Mash Piano 是给宝宝的钢琴玩具。拍一下、点一下、一把抓一堆键，弹出来的都是轻柔的钢琴声，而且每个音放在一起都好听。</p>'),
    ('<span id="dl-main-label">Download for computer</span><small id="dl-main-sub">Mac and Windows, free</small>',
     '<span id="dl-main-label">下载电脑版</span><small id="dl-main-sub">Mac 和 Windows，免费</small>'),
    ('<span><small>Download on the</small>App Store</span>', '<span><small>前往</small>App Store 下载</span>'),
    ('<a href="https://github.com/HeroSizy/baby-piano/releases/latest">All files</a>', '<a href="https://github.com/HeroSizy/baby-piano/releases/latest">全部文件</a>'),
    ('In WeChat, downloads may not start. Tap ··· and choose Open in Browser.', '在微信里可能无法下载。请点右上角 ···，选择“在浏览器打开”。'),
    ('Go on, have a go: tap or drag across the keys, or mash your keyboard.', '来试试：点一点或者划过琴键，也可以直接乱按电脑键盘。'),
    ('aria-label="A playable piano: tap or drag across the keys"', 'aria-label="可以弹的钢琴：点一点或者划过琴键"'),
    ('Sound is on. Real grand piano notes from a Yamaha C5.', '请打开声音。真实的三角钢琴音色，录自一台 Yamaha C5。'),
    # story
    ('<p>It started with an old mechanical keyboard. Our baby loved slapping it, so it became a toy.</p>',
     '<p>一切从一把旧机械键盘开始。宝宝特别爱拍它，它就成了玩具。</p>'),
    ('<p>But plugged into a real computer, every slap did something. Apps quit, windows switched, shortcuts fired. And none of it sounded nice.</p>',
     '<p>可是插到真电脑上，每拍一下都会出事：程序退出、窗口乱切、快捷键乱触发，而且一点也不好听。</p>'),
    ('<p>So we made it play music instead.</p>', '<p>所以我们让它改成弹音乐。</p>'),
    ('<p class="sig">— a parent, and the baby</p>', '<p class="sig">—— 一位家长，和宝宝</p>'),
    ('<div class="kicker">On a computer</div>', '<div class="kicker">在电脑上</div>'),
    ('<h2 class="sm">Every shortcut is just a note now</h2>', '<h2 class="sm">每个快捷键，现在都只是一个音</h2>'),
    ('<p class="big">The keyboard is locked while the baby plays. Nothing quits, switches or opens. Every key, even Esc and the volume keys, simply plays.</p>',
     '<p class="big">宝宝弹琴时整块键盘都被锁住：不会退出、不会切换、不会打开任何东西。每个键，连 Esc 和音量键，都只会弹出一个音。</p>'),
    ('<small class="was">Quit</small>', '<small class="was">退出</small>'),
    ('<small class="was">Switch</small>', '<small class="was">切换</small>'),
    ('<small class="was">Start</small>', '<small class="was">开始菜单</small>'),
    ('<small class="was">Close</small>', '<small class="was">关闭</small>'),
    # sound
    ('<div class="kicker">Made for little ears</div>', '<div class="kicker">为小耳朵设计</div>'),
    ('<h2>Built around the sound</h2>', '<h2>一切围绕声音</h2>'),
    ('<p class="big">At six months, babies mostly listen. So the sound comes first, and everything else stays out of the way.</p>',
     '<p class="big">六个月大的宝宝主要靠听。所以声音放在第一位，其他的都不打扰。</p>'),
    ('<h3>There are no wrong notes</h3>', '<h3>没有错音</h3>'),
    ('<p>Every key plays from one gentle scale: do re mi sol la, from deep bass to tinkling top. Mash ten keys at once and it still sounds like music. Grown-ups can switch to seven notes or all twelve.</p>',
     '<p>每个键都来自同一个温柔的五声音阶：do re mi sol la，从低沉的低音到清脆的高音。一次按十个键，听起来还是音乐。家长也可以换成七个音或全部十二个音。</p>'),
    ('<span>Low notes</span><span>High notes</span>', '<span>低音</span><span>高音</span>'),
    ('<h3>Soft where it matters</h3>', '<h3>该轻的地方都很轻</h3>'),
    ("<p>High notes play quieter than low ones. A slap across a dozen keys plays only a few. The computer's volume stays capped while playing, even if headphones or a speaker connect midway.</p>",
     '<p>高音比低音更轻。一巴掌拍到十几个键，也只会响几个。弹琴时电脑音量有上限，中途接上耳机或音箱也一样。</p>'),
    ('<h3>A calm screen, or fireworks</h3>', '<h3>安静的屏幕，或者烟花</h3>'),
    ('<p>By default the screen stays dark, because the baby is listening, not watching. Or turn on fireworks and every note bursts into color, like the piano above.</p>',
     '<p>默认屏幕保持黑色，因为宝宝是在听，不是在看。也可以打开烟花，每个音都会绽放出颜色，就像上面的钢琴一样。</p>'),
    # get it
    ('<div class="kicker">Free, everywhere</div>', '<div class="kicker">全平台免费</div>'),
    ('<h2>Get Mash Piano</h2>', '<h2>下载 Mash Piano</h2>'),
    ('<p class="big">The same toy on every device in the house, in 25 languages. No ads, no purchases, no accounts, and it never goes online.</p>',
     '<p class="big">家里每台设备上都是同一个玩具，支持 25 种语言。没有广告、没有内购、不用注册，也从不联网。</p>'),
    ('<span class="desc">Signed and notarized. Locks the keyboard.</span><span class="go">Download ↓</span>',
     '<span class="desc">已签名并经 Apple 公证。会锁住键盘。</span><span class="go">下载 ↓</span>'),
    ('<span class="desc">No admin rights needed. Locks the keyboard.</span><span class="go">Download ↓</span>',
     '<span class="desc">不需要管理员权限。会锁住键盘。</span><span class="go">下载 ↓</span>'),
    ('<span class="desc">Big keys for every finger, or plug in a keyboard.</span><span class="go">App Store →</span>',
     '<span class="desc">每根手指都能按的大按键，也可以接键盘。</span><span class="go">App Store →</span>'),
    ('<span class="desc">Phones and tablets. Install the APK.</span><span class="go">Download ↓</span>',
     '<span class="desc">手机和平板，安装 APK 即可。</span><span class="go">下载 ↓</span>'),
    ('<b>All files</b><span class="desc" id="ver">Every installer and what\'s new.</span><span class="go">Releases →</span>',
     '<b>全部文件</b><span class="desc" id="ver">所有安装包和更新内容。</span><span class="go">Releases →</span>'),
    ('<p class="alt">Mac with an Intel chip? <a href="/download/mac-x64">Intel version</a>. Windows on ARM? <a href="/download/windows-arm64">ARM version</a>.</p>',
     '<p class="alt">Intel 芯片的 Mac？<a href="/download/mac-x64">Intel 版</a>。ARM 的 Windows 电脑？<a href="/download/windows-arm64">ARM 版</a>。</p>'),
    # grown-ups
    ('<div class="kicker">For grown-ups</div>', '<div class="kicker">给家长</div>'),
    ("<h2>The baby can't get out.<br>You can.</h2>", '<h2>宝宝出不去，<br>你可以。</h2>'),
    ('<h3>Type the secret word</h3>', '<h3>输入暗号</h3>'),
    ('<p>On a keyboard, type it with no more than three seconds between letters.</p>', '<p>在键盘上输入下面这个词，每个字母之间不超过三秒。</p>'),
    ('<h3>Or hold the corner</h3>', '<h3>或者按住右上角</h3>'),
    ('<p>Hold a finger, or the mouse, in the top-right corner for three seconds.</p>', '<p>用手指或鼠标在屏幕右上角按住三秒。</p>'),
    ('<h3>Good to know</h3>', '<h3>小贴士</h3>'),
    ('<li><b>Mac:</b> the first time, allow Mash Piano under System Settings › Privacy &amp; Security › Accessibility. That is what lets it lock the keyboard. It records nothing.</li>',
     '<li><b>Mac：</b>第一次使用时，在 系统设置 › 隐私与安全性 › 辅助功能 里打开 Mash Piano。这是锁住键盘所需的权限，它不会记录任何内容。</li>'),
    ('<li><b>Laptops:</b> turn on "Also lock the mouse and trackpad" so little palms can\'t click anything.</li>',
     '<li><b>笔记本：</b>打开“同时锁住鼠标和触控板”，小手掌就什么都点不了。</li>'),
    ('<li><b>iPhone and iPad:</b> turn on Guided Access, then triple-click the side button while it plays. <b>Android:</b> use App pinning.</li>',
     '<li><b>iPhone 和 iPad：</b>打开“引导式访问”，弹琴时连按三下侧边按钮。<b>Android：</b>使用“应用固定”。</li>'),
    ('<span>No ads</span><span>No purchases</span><span>No accounts</span><span>No data collected</span><span>Works offline</span><span>25 languages</span>',
     '<span>没有广告</span><span>没有内购</span><span>不用注册</span><span>不收集数据</span><span>离线可用</span><span>25 种语言</span>'),
    # footer
    ('<span>Piano notes: <a href="https://archive.org/details/SalamanderGrandPianoV3">Salamander Grand Piano V3</a> by Alexander Holm, CC BY 3.0.</span>',
     '<span>钢琴音源：<a href="https://archive.org/details/SalamanderGrandPianoV3">Salamander Grand Piano V3</a>，作者 Alexander Holm，CC BY 3.0。</span>'),
    ('<a href="/privacy">Privacy</a><a href="https://github.com/HeroSizy/baby-piano">Source</a><a href="https://github.com/HeroSizy/baby-piano/issues">Support</a>',
     '<a href="/privacy">隐私</a><a href="https://github.com/HeroSizy/baby-piano">源代码</a><a href="https://github.com/HeroSizy/baby-piano/issues">支持</a>'),
]

def main():
    html = SRC.read_text(encoding="utf-8")
    missing = [en for en, _ in PAIRS if en not in html]
    if missing:
        sys.exit("Not in index.html any more:\n" + "\n".join("  " + m for m in missing))
    for en, zh in PAIRS:
        html = html.replace(en, zh)
    OUT.parent.mkdir(exist_ok=True)
    OUT.write_text("<!-- Generated by site/i18n/zh.py from index.html. Do not edit. -->\n" + html, encoding="utf-8")
    print("wrote", OUT)

main()
