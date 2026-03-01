// wsgate 命名空间，避免全局变量污染
var wsgate = wsgate || {}

// 检测浏览器是否支持完整的 console 调试接口（debug/info/warn/error）
wsgate.hasconsole = (typeof console !== 'undefined' && 'debug' in console && 'info' in console && 'warn' in console && 'error' in console);

/**
 * 将任意 JS 对象转换为可读字符串，用于调试输出。
 * 通过 depth 数组追踪已访问对象，检测并避免循环引用导致的无限递归。
 * @param {*}     obj   要转换的对象
 * @param {Array} depth 已遍历对象的引用栈（内部递归使用）
 * @returns {string} 对象的字符串表示
 */
wsgate.o2s = function(obj, depth) {
    depth = depth || [];
    // 检测循环引用，直接返回占位符
    if (depth.contains(obj)) {
        return '{SELF}';
    }
    switch (typeof(obj)) {
        case 'undefined':
            return 'undefined';
        case 'string':
            // 转义控制字符及反斜杠/引号
            return '"' + obj.replace(/[\x00-\x1f\\"]/g, escape) + '"';
        case 'array':
            var string = [];
            depth.push(obj);
            for (var i = 0; i < obj.length; ++i) {
                string.push(wsgate.o2s(obj[i], depth));
            }
            depth.pop();
            return '[' + string + ']';
        case 'object':
        case 'hash':
            var string = [];
            depth.push(obj);
            // UIEvent 某些属性（layerX/layerY/view）访问会抛异常，需特殊处理
            var isE = (obj instanceof UIEvent);
            Object.each(obj, function(v, k) {
                if (v instanceof HTMLElement) {
                    // HTMLElement 不递归展开，直接标注类型
                    string.push(k + '={HTMLElement}');
                } else if (isE && (('layerX' == k) || ('layerY' == k) ('view' == k))) {
                    // UI 事件危险属性，跳过序列化
                    string.push(k + '=!0');
                } else {
                    try {
                        var vstr = wsgate.o2s(v, depth);
                        if (vstr) {
                            string.push(k + '=' + vstr);
                        }
                    } catch (error) {
                        // 序列化失败时用占位符标记
                        string.push(k + '=??E??');
                    }
                }
            });
            depth.pop();
            return '{' + string + '}';
        case 'number':
        case 'boolean':
            return '' + obj;
        case 'null':
            return 'null';
    }
    return null;
};

/**
 * 日志类：同时向浏览器控制台和 WebSocket 输出日志（用于远程日志收集）。
 */
wsgate.Log = new Class({
    /** 初始化：WebSocket 引用置空 */
    initialize: function() {
        this.ws = null;
    },
    /**
     * 内部方法：将参数列表拼成字符串后经 WebSocket 发送。
     * @param {string}    pfx 日志级别前缀，如 'D:' / 'I:' / 'W:' / 'E:'
     * @param {Arguments} a   需要记录的参数列表
     */
    _p: function(pfx, a) {
        var line = '';
        var i;
        for (i = 0; i < a.length; ++i) {
            switch (typeof(a[i])) {
                case 'string':
                case 'number':
                case 'boolean':
                case 'null':
                    line += a[i] + ' ';
                    break;
                default:
                    // 复杂对象调用 o2s 序列化为字符串
                    line += wsgate.o2s(a[i]) + ' ';
                    break;
            }
        }
        if (0 < line.length) {
            this.ws.send(pfx + line);
        }
    },
    /** 空操作：用于在不需要日志时替换其他方法（丢弃日志） */
    drop: function() {
    },
    /** 输出 DEBUG 级别日志（同时发往 WebSocket 和浏览器控制台） */
    debug: function() {/* DEBUG */
        if (this.ws) {
            this._p('D:', arguments);
        }
        if (wsgate.hasconsole) {
            try { console.debug.apply(this, arguments); } catch (error) { }
        }
        /* /DEBUG */},
    /** 输出 INFO 级别日志 */
    info: function() {
        if (this.ws) {
            var a = Array.prototype.slice.call(arguments);
            a.unshift('I:');
            this._p.apply(this, a);
        }
        /* DEBUG */if (wsgate.hasconsole) {
            try { console.info.apply(this, arguments); } catch (error) { }
        }/* /DEBUG */
    },
    /** 输出 WARN 级别日志 */
    warn: function() {
        if (this.ws) {
            var a = Array.prototype.slice.call(arguments);
            a.unshift('W:');
            this._p.apply(this, a);
        }
        /* DEBUG */if (wsgate.hasconsole) {
            try { console.warn.apply(this, arguments); } catch (error) { }
        }/* /DEBUG */
    },
    /** 输出 ERROR 级别日志 */
    err: function() {
        if (this.ws) {
            var a = Array.prototype.slice.call(arguments);
            a.unshift('E:');
            this._p.apply(this, a);
        }
        /* DEBUG */if (wsgate.hasconsole) {
            try { console.error.apply(this, arguments); } catch (error) { }
        }/* /DEBUG */
    },
    /**
     * 设置用于远程传输日志的 WebSocket 实例。
     * 传入 null 则关闭远程日志。
     */
    setWS: function(_ws) {
        this.ws = _ws;
    }
});

/**
 * WebSocket 连接基类，封装 WebSocket 的创建与四个事件的绑定。
 * 子类需实现 onWSopen / onWSclose / onWSmsg / onWSerr 方法。
 */
wsgate.WSrunner = new Class( {
    Implements: Events,
    /**
     * 构造函数。
     * @param {string} url WebSocket 服务器地址
     */
    initialize: function(url) {
        this.url = url;
    },
    /**
     * 建立 WebSocket 连接并绑定事件处理器。
     * 使用 arraybuffer 模式接收二进制数据（RDP 位图指令）。
     */
    Run: function() {
        try {
            this.sock = new WebSocket(this.url);
        } catch (err) { }
        // 二进制数据以 ArrayBuffer 形式接收，方便用 TypedArray 解析
        this.sock.binaryType = 'arraybuffer';
        this.sock.onopen    = this.onWSopen.bind(this);
        this.sock.onclose   = this.onWSclose.bind(this);
        this.sock.onmessage = this.onWSmsg.bind(this);
        this.sock.onerror   = this.onWSerr.bind(this);
    }
});

/**
 * RDP 客户端主类，继承自 WSrunner。
 * 负责将 WebSocket 收到的 RDP 绘图指令渲染到 Canvas，
 * 并将用户鼠标/键盘/触摸输入经 WebSocket 发回服务端。
 */
wsgate.RDP = new Class( {
    Extends: wsgate.WSrunner,
    /**
     * 构造函数，初始化 Canvas、离屏缓冲、光标及所有状态变量。
     * @param {string}  url       WebSocket 服务器地址
     * @param {Element} canvas    渲染 RDP 画面的 Canvas 元素
     * @param {boolean} cssCursor true=用 CSS cursor 显示光标；false=用 <img> 元素模拟
     * @param {boolean} useTouch  是否启用触摸事件支持
     * @param {Object}  vkbd      虚拟键盘对象（可选，提供 vkpress 事件）
     */
    initialize: function(url, canvas, cssCursor, useTouch, vkbd) {
        this.log = new wsgate.Log();
        this.canvas = canvas;
        // 主绘图上下文，所有 RDP 绘图指令都输出到这里
        this.cctx = canvas.getContext('2d');
        this.cctx.strokeStyle = 'rgba(255,255,255,0)';
        this.cctx.FillStyle = 'rgba(255,255,255,0)';
        // 离屏缓冲 Canvas：putImageData 不遵守裁剪区域，
        // 需要裁剪时先画到缓冲区，再用 drawImage（遵守裁剪）贴到主 Canvas
        this.bstore = new Element('canvas', {
            'width':this.canvas.width,
            'height':this.canvas.height,
        });
        this.bctx = this.bstore.getContext('2d');
        this.aMF = 0;       // 人工鼠标标志位（0=禁用，用于触摸屏模拟鼠标按键）
        this.Tcool = true;  // 触摸冷却标志：多指触摸后需冷却，防止误触
        this.pTe = null;    // 暂存待处理的触摸事件（等待防抖定时器）
        this.ccnt = 0;      // Canvas save/restore 嵌套计数，确保调用平衡
        // 当前裁剪区域坐标（由 SetBounds 指令设置）
        this.clx = 0;
        this.cly = 0;
        this.clw = 0;
        this.clh = 0;
        // 当前鼠标坐标（用于更新自定义光标图片位置）
        this.mX = 0;
        this.mY = 0;
        // 光标热点坐标（cursor hotspot，光标图片中的点击基准点）
        this.chx = 10;
        this.chy = 10;
        // 需要特殊处理的修饰键键码列表：
        // Backspace(8) Shift(16) Ctrl(17) Alt(18) CapsLock(20) NumLock(144) ScrollLock(145)
        this.modkeys = [8, 16, 17, 18, 20, 144, 145];
        this.cursors = new Array(); // 光标缓存：id -> CSS cursor 字符串或 {u,x,y} 对象
        this.sid = null;            // 会话 ID，由服务端 'S:' 消息下发，用于构造光标图片 URL
        this.open = false;          // 标记 WebSocket 是否已成功连接
        this.cssC = cssCursor;
        this.uT = useTouch;
        if (!cssCursor) {
            // 用 <img> 元素模拟光标，绝对定位覆盖在 Canvas 上方（z-index:998）
            this.cI = new Element('img', {
                'src': '/c_default.png',
                'styles': {
                    'position': 'absolute',
                'z-index': 998,
                'left': this.mX - this.chx,
                'top': this.mY - this.chy
                }
            }).inject(document.body);
        }
        if (vkbd) {
            // 监听虚拟键盘按键事件
            vkbd.addEvent('vkpress', this.onKv.bind(this));
        }
        this.parent(url);
    },
    /** 主动断开 RDP 连接（重置所有状态） */
    Disconnect: function() {
        this._reset();
    },
    /**
     * 更新光标 <img> 元素的位置，使其跟随鼠标坐标移动。
     * 需减去热点偏移（chx/chy），使光标热点对齐鼠标实际坐标。
     */
    cP: function() {
        this.cI.setStyles({'left': this.mX - this.chx, 'top': this.mY - this.chy});
    },
    /**
     * 检查点 (x, y) 是否在当前裁剪区域内。
     * 若裁剪区域宽高均为 0，表示无裁剪限制，始终返回 true。
     * @returns {boolean}
     */
    _ckclp: function(x, y) {
        if (this.clw || this.clh) {
            return (
                    (x >= this.clx) &&
                    (x <= (this.clx + this.clw)) &&
                    (y >= this.cly) &&
                    (y <= (this.cly + this.clh))
                   );
        }
        // 未设置裁剪区域，不限制绘制范围
        return true;
    },
    /**
     * 二进制 RDP 消息处理主循环。
     * 根据消息首个 uint32（操作码）分发到对应绘图或光标处理逻辑。
     * 消息格式：[uint32 op | uint32[] 参数 | 可变长度数据]
     * @param {ArrayBuffer} data WebSocket 收到的二进制消息
     */
    _pmsg: function(data) { // process a binary RDP message from our queue
        var op, hdr, count, rects, bmdata, rgba, compressed, i, offs, x, y, sx, sy, w, h, dw, dh, bpp, color, len;
        op = new Uint32Array(data, 0, 1);
        switch (op[0]) {
            case 0:
                // op=0 BeginPaint：开始绘制，保存 Canvas 状态
                // this.log.debug('BeginPaint');
                this._ctxS();
                break;
            case 1:
                // op=1 EndPaint：结束绘制，恢复 Canvas 状态
                // this.log.debug('EndPaint');
                this._ctxR();
                break;
            case 2:
                // op=2 单张位图绘制（Single Bitmap）
                // 消息头（偏移 4 字节起，共 9 个 uint32）：
                //  [0]=目标X  [1]=目标Y  [2]=源宽  [3]=源高
                //  [4]=目标宽 [5]=目标高 [6]=BPP   [7]=是否压缩  [8]=数据字节数
                hdr = new Uint32Array(data, 4, 9);
                bmdata = new Uint8Array(data, 40); // 位图像素数据从偏移40开始
                x = hdr[0];
                y = hdr[1];
                w = hdr[2];
                h = hdr[3];
                dw = hdr[4];
                dh = hdr[5];
                bpp = hdr[6];
                compressed =  (hdr[7] != 0);
                len = hdr[8];
                if ((bpp == 16) || (bpp == 15)) {
                    if (this._ckclp(x, y) && this._ckclp(x + dw, y + dh)) {
                        // 目标区域完全在裁剪范围内，直接绘制到主 Canvas
                        // this.log.debug('BMi:',(compressed ? ' C ' : ' U '),' x=',x,'y=',y,' w=',w,' h=',h,' l=',len);
                        var outB = this.cctx.createImageData(w, h);
                        if (compressed) {
                            // RLE16 解压 + 垂直翻转（RDP 位图为 bottom-up 存储）
                            wsgate.dRLE16_RGBA(bmdata, len, w, outB.data);
                            wsgate.flipV(outB.data, w, h);
                        } else {
                            // 无压缩 RGB16 直接解码为 RGBA
                            wsgate.dRGB162RGBA(bmdata, len, outB.data);
                        }
                        this.cctx.putImageData(outB, x, y, 0, 0, dw, dh);
                    } else {
                        // this.log.debug('BMc:',(compressed ? ' C ' : ' U '),' x=',x,'y=',y,' w=',w,' h=',h,' bpp=',bpp);
                        // putImageData 不遵守 Canvas 裁剪区域，
                        // 因此先绘制到离屏缓冲，再用 drawImage（遵守裁剪）转贴到主 Canvas
                        var outB = this.bctx.createImageData(w, h);
                        if (compressed) {
                            wsgate.dRLE16_RGBA(bmdata, len, w, outB.data);
                            wsgate.flipV(outB.data, w, h);
                        } else {
                            wsgate.dRGB162RGBA(bmdata, len, outB.data);
                        }
                        this.bctx.putImageData(outB, 0, 0, 0, 0, dw, dh);
                        this.cctx.drawImage(this.bstore, 0, 0, dw, dh, x, y, dw, dh);
                    }
                } else {
                    this.log.warn('BPP <> 15/16 not yet implemented');
                }
                break;
            case 3:
                // op=3 OPAQUE_RECT_ORDER：不透明颜色填充矩形
                // 数据：[x, y, w, h]（Int32） + [R,G,B,A]（Uint8）
                hdr = new Int32Array(data, 4, 4);
                rgba = new Uint8Array(data, 20, 4);
                // this.log.debug('Fill:',hdr[0], hdr[1], hdr[2], hdr[3], this._c2s(rgba));
                this.cctx.fillStyle = this._c2s(rgba);
                this.cctx.fillRect(hdr[0], hdr[1], hdr[2], hdr[3]);
                break;
            case 4:
                // op=4 SetBounds：设置裁剪区域
                // 数据：[left, top, right, bottom]，需转换为 x,y,w,h
                hdr = new Int32Array(data, 4, 4);
                this._cR(hdr[0], hdr[1], hdr[2] - hdr[0], hdr[3] - hdr[1], true);
                break;
            case 5:
                // op=5 PatBlt：图案位块传输（Pattern BLT）
                if (28 == data.byteLength) {
                    // 实心画刷，数据：[x, y, w, h]（Int32）+ [R,G,B,A]（Uint8）+ rop3（Uint32）
                    hdr = new Int32Array(data, 4, 4);
                    x = hdr[0];
                    y = hdr[1];
                    w = hdr[2];
                    h = hdr[3];
                    rgba = new Uint8Array(data, 20, 4);
                    this._ctxS();
                    this._cR(x, y, w, h, false);
                    if (this._sROP(new Uint32Array(data, 24, 1)[0])) {
                        this.cctx.fillStyle = this._c2s(rgba);
                        this.cctx.fillRect(x, y, w, h);
                    }
                    this._ctxR();
                } else {
                    this.log.warn('PatBlt: Patterned brush not yet implemented');
                }
                break; 
            case 6:
                // op=6 Multi Opaque rect：批量不透明矩形填充
                // 数据：[R,G,B,A] + nrects（Uint32）+ rect0.x,y,w,h ... rectN.x,y,w,h
                rgba = new Uint8Array(data, 4, 4);
                count = new Uint32Array(data, 8, 1);
                rects = new Uint32Array(data, 12, count[0] * 4);
                // this.log.debug('MultiFill: ', count[0], " ", this._c2s(rgba));
                this.cctx.fillStyle = this._c2s(rgba);
                offs = 0;
                for (i = 0; i < count[0]; ++i) {
                    this.cctx.fillRect(rects[offs], rects[offs+1], rects[offs+2], rects[offs+3]);
                    offs += 4; // 每个矩形占 4 个 uint32
                }
                break;
            case 7:
                // op=7 ScrBlt：屏幕位块传输，将屏幕某区域复制到另一位置
                // 数据：rop3（Uint32）+ [x,y,w,h,sx,sy]（Int32），sx/sy 为源坐标
                hdr = new Int32Array(data, 8, 6);
                x = hdr[0];
                y = hdr[1];
                w = hdr[2];
                h = hdr[3];
                sx = hdr[4]; // 源区域左上角 X
                sy = hdr[5]; // 源区域左上角 Y
                if ((w > 0) && (h > 0)) {
                    if (this._sROP(new Uint32Array(data, 4, 1)[0])) {
                        if (this._ckclp(x, y) && this._ckclp(x + w, y + h)) {
                            // 目标在裁剪范围内，直接 getImageData/putImageData
                            this.cctx.putImageData(this.cctx.getImageData(sx, sy, w, h), x, y);
                        } else {
                            // 需要裁剪：借助离屏缓冲 + drawImage
                            this.bctx.putImageData(this.cctx.getImageData(sx, sy, w, h), 0, 0);
                            this.cctx.drawImage(this.bstore, 0, 0, w, h, x, y, w, h);
                        }
                    }
                } else {
                    this.log.warn('ScrBlt: width and/or height is zero');
                }
                break;
            case 8:
                // op=8 PTR_NEW：服务端下发新光标，含 id/热点坐标
                // 数据：[id, xhot, yhot]（Uint32）
                hdr = new Uint32Array(data, 4, 3);
                if (this.cssC) {
                    // CSS cursor 模式：构造含热点的 url() 字符串缓存
                    this.cursors[hdr[0]] = 'url(/cur/'+this.sid+'/'+hdr[0]+') '+hdr[1]+' '+hdr[2]+',none';
                } else {
                    // img 元素模式：缓存图片 URL 和热点坐标
                    this.cursors[hdr[0]] = {u: '/cur/'+this.sid+'/'+hdr[0], x: hdr[1], y: hdr[2]};
                }
                break;
            case 9:
                // op=9 PTR_FREE：删除指定 id 的光标缓存
                this.cursors[new Uint32Array(data, 4, 1)[0]] = undefined;
                break;
            case 10:
                // op=10 PTR_SET：切换到指定 id 的光标
                // this.log.debug('PS:', this.cursors[new Uint32Array(data, 4, 1)[0]]);
                if (this.cssC) {
                    this.canvas.setStyle('cursor', this.cursors[new Uint32Array(data, 4, 1)[0]]);
                } else {
                    var cobj = this.cursors[new Uint32Array(data, 4, 1)[0]];
                    this.chx = cobj.x; // 更新热点 X
                    this.chy = cobj.y; // 更新热点 Y
                    this.cI.src = cobj.u;
                }
                break;
            case 11:
                // op=11 PTR_SETNULL：隐藏光标
                if (this.cssC) {
                    this.canvas.setStyle('cursor', 'none');
                } else {
                    this.cI.src = '/c_none.png';
                }
                break;
            case 12:
                // op=12 PTR_SETDEFAULT：恢复默认光标
                if (this.cssC) {
                    this.canvas.setStyle('cursor', 'default');
                } else {
                    this.chx = 10;
                    this.chy = 10;
                    this.cI.src = '/c_default.png';
                }
                break;
            default:
                this.log.warn('Unknown BINRESP: ', data.byteLength);
        }
    },
    /**
     * 设置 Canvas 裁剪区域（直接替换，不做交集运算）。
     * @param {number}  x    裁剪区左上角 X
     * @param {number}  y    裁剪区左上角 Y
     * @param {number}  w    裁剪区宽度
     * @param {number}  h    裁剪区高度
     * @param {boolean} save 是否将区域保存到对象属性（供 _ckclp 使用）
     */
    _cR: function(x, y, w, h, save) {
        if (save) {
            this.clx = x;
            this.cly = y;
            this.clw = w;
            this.clh = h;
        }
        // 先重置为全画布裁剪，再设置新区域（不取交集）
        this.cctx.beginPath();
        this.cctx.rect(0, 0, this.canvas.width, this.canvas.height);
        this.cctx.clip();
        if (x == y == 0) {
            // x、y 均为 0 且宽高为 0 或等于画布尺寸，则表示重置为全画布，直接返回
            if ((w == h == 0) || ((w == this.canvas.width) && (h == this.canvas.height))) {
                return;
            }
        }
        // 应用新的裁剪矩形
        this.cctx.beginPath();
        this.cctx.rect(x, y, w, h);
        this.cctx.clip();
    },
    /**
     * 根据 RDP ROP3 操作码设置 Canvas 合成混合模式。
     * 仅支持三种常用 ROP3，不支持的操作码输出警告并返回 false。
     * @param {number} rop RDP ROP3 操作码（32位）
     * @returns {boolean} 受支持返回 true
     */
    _sROP: function(rop) {
        switch (rop) {
            case 0x005A0049:
                // GDI_PATINVERT: D = P ^ D（目标与画刷异或）
                this.cctx.globalCompositeOperation = 'xor';
                return true;
                break;
            case 0x00F00021:
                // GDI_PATCOPY: D = P（画刷覆盖目标）
                this.cctx.globalCompositeOperation = 'copy';
                return true;
                break;
            case 0x00CC0020:
                // GDI_SRCCOPY: D = S（源图覆盖目标）
                this.cctx.globalCompositeOperation = 'source-over';
                return true;
                break;
            default:
                this.log.warn('Unsupported raster op: ', rop.toString(16));
                break;
        }
        return false;
        /*
           case 0x00EE0086:
        // GDI_SRCPAINT: D = S | D
        break;
        case 0x008800C6:
        // GDI_SRCAND: D = S & D
        break;
        case 0x00660046:
        // GDI_SRCINVERT: D = S ^ D
        break;
        case 0x00440328:
        // GDI_SRCERASE: D = S & ~D
        break;
        case 0x00330008:
        // GDI_NOTSRCCOPY: D = ~S
        break;
        case 0x001100A6:
        // GDI_NOTSRCERASE: D = ~S & ~D
        break;
        case 0x00C000CA:
        // GDI_MERGECOPY: D = S & P
        break;
        case 0x00BB0226:
        // GDI_MERGEPAINT: D = ~S | D
        break;
        case 0x00FB0A09:
        // GDI_PATPAINT: D = D | (P | ~S)
        break;
        case 0x00550009:
        // GDI_DSTINVERT: D = ~D
        break;
        case 0x00000042:
        // GDI_BLACKNESS: D = 0
        break;
        case 0x00FF0062:
        // GDI_WHITENESS: D = 1
        break;
        case 0x00E20746:
        // GDI_DSPDxax: D = (S & P) | (~S & D)
        break;
        case 0x00B8074A:
        // GDI_PSDPxax: D = (S & D) | (~S & P)
        break;
        case 0x000C0324:
        // GDI_SPna: D = S & ~P
        break;
        case 0x00220326:
        // GDI_DSna D = D & ~S
        break;
        case 0x00220326:
        // GDI_DSna: D = D & ~S
        break;
        case 0x00A000C9:
        // GDI_DPa: D = D & P
        break;
        case 0x00A50065:
        // GDI_PDxn: D = D ^ ~P
        break;
        */
    },
    /**
     * 重置连接状态（断开 WebSocket、清空画布、移除所有事件监听）。
     * 主动断开或连接异常时调用。
     */
    _reset: function() {
        this.log.setWS(null); // 关闭远程日志传输
        if (this.sock.readyState == this.sock.OPEN) {
            this.fireEvent('disconnected');
            this.sock.close();
        }
        // 清空裁剪区域记录
        this.clx = 0;
        this.cly = 0;
        this.clw = 0;
        this.clh = 0;
        this.canvas.removeEvents();
        document.removeEvents();
        // 平衡 save/restore，防止 Canvas 状态泄漏
        while (this.ccnt > 0) {
            this.cctx.restore();
            this.ccnt -= 1;
        }
        this.cctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        // 更新页面标题为离线状态
        document.title = document.title.replace(/:.*/, ': offline');
        if (this.cssC) {
            this.canvas.setStyle('cursor','default');
        } else {
            this.cI.src = '/c_default.png';
        }
        if (!this.cssC) {
            // 销毁自定义光标图片元素
            this.cI.removeEvents();
            this.cI.destroy();
        }
    },
    /**
     * 触摸防抖回调：延迟 50ms 后决定是触发手势事件还是模拟鼠标按下。
     * 防止单指轻触被误判为多指手势。
     */
    fT: function() {
        delete this.fTid;
        if (this.pT) {
            // 多指触摸：触发对应手势事件（touch2/touch3/touch4）
            this.fireEvent('touch' + this.pT);
            this.pT = 0;
            return;
        }
        if (this.pTe) {
            // 单指触摸：模拟鼠标按下
            this.onMd(this.pTe);
            this.pTe = null;
        }
    },
    /**
     * 重置触摸冷却标志，允许再次响应触摸点击。
     * 多指触摸后等待 500ms 再恢复，避免误触。
     */
    cT: function() {
        this.log.debug('cT');
        this.Tcool = true;
    },
    /**
     * 触摸开始事件处理器（touchstart）。
     * 单指：延迟判断是否为鼠标按下；多指（2~4 指）：触发对应手势事件。
     * @param {TouchEvent} evt 触摸事件
     */
    onTs: function(evt) {
        var tn = evt.targetTouches.length;
        this.log.debug('Ts:', tn);
        switch (tn) {
            default:
                break;
            case 1:
                // 单指：暂存事件，50ms 后决策（等待可能的多指抬起）
                this.pTe = evt;
                evt.preventDefault();
                if ('number' == typeof(this.fTid)) {
                    clearTimeout(this.fTid);
                }
                this.fTid = this.fT.delay(50, this);
                break;
            case 2:
                // 双指手势，关闭冷却，500ms 后恢复
                this.pT = 2;
                this.Tcool = false;
                evt.preventDefault();
                if ('number' == typeof(this.fTid)) {
                    clearTimeout(this.fTid);
                }
                this.cT.delay(500, this)
                this.fTid = this.fT.delay(50, this);
                break;
            case 3:
                this.pT = 3;
                this.Tcool = false;
                evt.preventDefault();
                if ('number' == typeof(this.fTid)) {
                    clearTimeout(this.fTid);
                }
                this.cT.delay(500, this)
                this.fTid = this.fT.delay(50, this);
                break;
            case 4:
                this.pT = 4;
                this.Tcool = false;
                evt.preventDefault();
                if ('number' == typeof(this.fTid)) {
                    clearTimeout(this.fTid);
                }
                this.cT.delay(500, this)
                this.fTid = this.fT.delay(50, this);
                break;
        }
        return true;
    },
    /**
     * 触摸结束事件处理器（touchend）。
     * 单指抬起且冷却期内，模拟鼠标松开。
     * @param {TouchEvent} evt 触摸事件
     */
    onTe: function(evt) {
        if ((0 == evt.targetTouches.length) && this.Tcool) {
            evt.preventDefault();
            // 用 changedTouches 获取最后离开屏幕的坐标
            this.onMu(evt, evt.changedTouches[0].pageX, evt.changedTouches[0].pageY);
        }
    },
    /**
     * 触摸移动事件处理器（touchmove）。
     * 单指移动时转发为鼠标移动事件。
     * @param {TouchEvent} evt 触摸事件
     */
    onTm: function(evt) {
        // this.log.debug('Tm:', evt);
        if (1 == evt.targetTouches.length) {
            this.onMm(evt);
        }
    },
    /**
     * 鼠标移动事件处理器（mousemove）。
     * 打包 WSOP_CS_MOUSE + PTR_FLAGS_MOVE 消息发往服务端，并更新自定义光标位置。
     * @param {Event} evt MooTools 封装的鼠标事件
     */
    onMm: function(evt) {
        var buf, a, x, y;
        evt.preventDefault();
        x = evt.event.layerX;
        y = evt.event.layerY;
        if (!this.cssC) {
            // 更新自定义光标 <img> 的位置
            this.mX = x;
            this.mY = y;
            this.cP();
        }
        // this.log.debug('mM x: ', x, ' y: ', y);
        if (this.sock.readyState == this.sock.OPEN) {
            buf = new ArrayBuffer(16);
            a = new Uint32Array(buf);
            a[0] = 0;      // 消息类型：WSOP_CS_MOUSE
            a[1] = 0x0800; // PTR_FLAGS_MOVE（鼠标移动标志）
            a[2] = x;
            a[3] = y;
            this.sock.send(buf);
        }
    },
    /**
     * 鼠标按下事件处理器（mousedown）。
     * 发送含按下标志（bit15=1，即 0x8000）的 WSOP_CS_MOUSE 消息。
     * 右键+Ctrl+Alt 组合触发三指手势事件。
     * @param {Event} evt MooTools 封装的鼠标事件
     */
    onMd: function(evt) {
        var buf, a, x, y, which;
        if (this.Tcool) {
            evt.preventDefault();
            // 右键 + Ctrl + Alt 组合视为三指手势，呼出菜单等
            if (evt.rightClick && evt.control && evt.alt) {
                this.fireEvent('touch3');
                return;
            }
            x = evt.event.layerX;
            y = evt.event.layerY;
            which = this._mB(evt); // 获取 RDP 鼠标按键标志位
            this.log.debug('mD b: ', which, ' x: ', x, ' y: ', y);
            if (this.sock.readyState == this.sock.OPEN) {
                buf = new ArrayBuffer(16);
                a = new Uint32Array(buf);
                a[0] = 0;              // 消息类型：WSOP_CS_MOUSE
                a[1] = 0x8000 | which; // 0x8000=按下标志 | 按键标志
                a[2] = x;
                a[3] = y;
                this.sock.send(buf);
            }
        }
    },
    /**
     * 鼠标松开事件处理器（mouseup）。
     * 发送不含按下标志的 WSOP_CS_MOUSE 消息（表示释放）。
     * @param {Event}  evt MooTools 封装的鼠标事件
     * @param {number} x   可选，覆盖事件坐标（触摸事件使用）
     * @param {number} y   可选，覆盖事件坐标（触摸事件使用）
     */
    onMu: function(evt, x, y) {
        var buf, a, x, y, which;
        if (this.Tcool) {
            evt.preventDefault();
            x = x || evt.event.layerX;
            y = y || evt.event.layerY;
            which = this._mB(evt);
            this.log.debug('mU b: ', which, ' x: ', x, ' y: ', y);
            if (this.aMF) {
                // 人工鼠标模式下触发释放事件，通知上层
                this.fireEvent('mouserelease');
            }
            if (this.sock.readyState == this.sock.OPEN) {
                buf = new ArrayBuffer(16);
                a = new Uint32Array(buf);
                a[0] = 0;     // 消息类型：WSOP_CS_MOUSE
                a[1] = which; // 仅按键标志，无 0x8000（表示释放）
                a[2] = x;
                a[3] = y;
                this.sock.send(buf);
            }
        }
    },
    /**
     * 鼠标滚轮事件处理器（mousewheel）。
     * 将滚轮方向转换为 RDP 滚轮标志发送（0x0200 | 向上 0x087 / 向下 0x188）。
     * @param {Event} evt MooTools 封装的滚轮事件（evt.wheel > 0 为向上）
     */
    onMw: function(evt) {
        var buf, a, x, y;
        evt.preventDefault();
        x = evt.event.layerX;
        y = evt.event.layerY;
        // this.log.debug('mW d: ', evt.wheel, ' x: ', x, ' y: ', y);
        if (this.sock.readyState == this.sock.OPEN) {
            buf = new ArrayBuffer(16);
            a = new Uint32Array(buf);
            a[0] = 0; // 消息类型：WSOP_CS_MOUSE
            // PTR_FLAGS_WHEEL(0x200) | 向上步长(0x087) 或 向下步长(0x188)
            a[1] = 0x200 | ((evt.wheel > 0) ? 0x087 : 0x188);
            a[2] = 0;
            a[3] = 0;
            this.sock.send(buf);
        }
    },
    /**
     * 键盘按下事件处理器（keydown）。
     * 仅处理修饰键（Shift/Ctrl/Alt 等），发送 WSOP_CS_KUPDOWN 按下消息。
     * @param {Event} evt MooTools 封装的键盘事件
     */
    onKd: function(evt) {
        var a, buf;
        if (this.modkeys.contains(evt.code)) {
            evt.preventDefault();
            // this.log.debug('kD code: ', evt.code, ' ', evt);
            if (this.sock.readyState == this.sock.OPEN) {
                buf = new ArrayBuffer(12);
                a = new Uint32Array(buf);
                a[0] = 1; // WSOP_CS_KUPDOWN
                a[1] = 1; // down
                a[2] = evt.code;
                this.sock.send(buf);
            }
        }
    },
    /**
     * 键盘松开事件处理器（keyup）。
     * 仅处理修饰键，发送 WSOP_CS_KUPDOWN 松开消息（a[1]=0 表示松开）。
     */
    onKu: function(evt) {
        var a, buf;
        if (this.modkeys.contains(evt.code)) {
            evt.preventDefault();
            // this.log.debug('kU code: ', evt.code);
            if (this.sock.readyState == this.sock.OPEN) {
                buf = new ArrayBuffer(12);
                a = new Uint32Array(buf);
                a[0] = 1; // WSOP_CS_KUPDOWN（修饰键按下/松开）
                a[1] = 0; // 0 = 松开
                a[2] = evt.code;
                this.sock.send(buf);
            }
        }
    },
    /**
     * 虚拟键盘按键事件处理器（vkpress）。
     * 特殊键（功能键等）发 WSOP_CS_KUPDOWN（先按下再松开）；
     * 普通字符发 WSOP_CS_KPRESS（含修饰键位掩码：bit0=Shift, bit1=Ctrl, bit2=Alt, bit3=Meta）。
     * @param {Object} evt 含 special/code/shift/control/alt/meta 字段的虚拟键盘事件
     */
    onKv: function(evt) {
        var a, buf;
        if (this.sock.readyState == this.sock.OPEN) {
            // this.log.debug('kP code: ', evt.code);
            buf = new ArrayBuffer(12);
            a = new Uint32Array(buf);
            if (evt.special) {
                // 特殊键：模拟按下后立即松开
                a[0] = 1; // WSOP_CS_KUPDOWN
                a[1] = 1; // 按下
                a[2] = evt.code;
                this.sock.send(buf);
                a[0] = 1; // WSOP_CS_KUPDOWN
                a[1] = 0; // 松开
                a[2] = evt.code;
            } else {
                // 普通字符：发 keypress 消息，含修饰键位掩码
                a[0] = 2; // WSOP_CS_KPRESS
                a[1] = (evt.shift ? 1 : 0)|(evt.control ? 2 : 0)|(evt.alt ? 4 : 0)|(evt.meta ? 8 : 0);
                a[2] = evt.code;
            }
            this.sock.send(buf);
        }
    },
    /**
     * 键盘字符按下事件处理器（keypress）。
     * 跳过修饰键（由 onKd/onKu 处理），将其他字符发送为 WSOP_CS_KPRESS 消息。
     */
    onKp: function(evt) {
        var a, buf;
        evt.preventDefault();
        // 修饰键已由 onKd/onKu 处理，此处直接跳过
        if (this.modkeys.contains(evt.code)) {
            return;
        }
        if (this.sock.readyState == this.sock.OPEN) {
            // this.log.debug('kP code: ', evt.code);
            buf = new ArrayBuffer(12);
            a = new Uint32Array(buf);
            a[0] = 2; // WSOP_CS_KPRESS
            // 修饰键位掩码：bit0=Shift, bit1=Ctrl, bit2=Alt, bit3=Meta
            a[1] = (evt.shift ? 1 : 0)|(evt.control ? 2 : 0)|(evt.alt ? 4 : 0)|(evt.meta ? 8 : 0);
            a[2] = evt.code;
            this.sock.send(buf);
        }
    },
    /**
     * WebSocket 消息接收事件处理器（onmessage）。
     * 文本消息为控制信令（告警/日志/会话ID等）；
     * 二进制消息（ArrayBuffer）为 RDP 绘图指令，转发给 _pmsg 处理。
     */
    onWSmsg: function(evt) {
        switch (typeof(evt.data)) {
            // 文本消息：控制信令，按前两字节前缀分发
            case 'string':
                // this.log.debug(evt.data);
                switch (evt.data.substr(0,2)) {
                    case "T:":
                            // T: 会话正常终止
                            this._reset();
                            break;
                    case "E:":
                            // E: 服务端错误，显示告警并断开
                            this.log.err(evt.data.substring(2));
                            this.fireEvent('alert', evt.data.substring(2));
                            this._reset();
                            break;
                    case 'I:':
                            // I: 普通信息日志
                            this.log.info(evt.data.substring(2));
                            break;
                    case 'W:':
                            // W: 警告日志
                            this.log.warn(evt.data.substring(2));
                            break;
                    case 'D:':
                            // D: 调试日志
                            this.log.debug(evt.data.substring(2));
                            break;
                    case 'S:':
                            // S: 会话 ID（用于构造光标图片 URL）
                            this.sid = evt.data.substring(2);
                            break;
                }
                break;
                // 二进制消息：RDP 绘图/光标指令
            case 'object':
                this._pmsg(evt.data);
                break;
        }

    },
    /**
     * WebSocket 连接建立事件处理器（onopen）。
     * 绑定鼠标/键盘/触摸事件，启用远程日志，并触发 connected 事件。
     */
    onWSopen: function(evt) {
        this.open = true;
        // 将 WebSocket 注入日志对象，启用远程日志传输
        this.log.setWS(this.sock);
        // 绑定 Canvas 输入事件
        this.canvas.addEvent('mousemove', this.onMm.bind(this));
        this.canvas.addEvent('mousedown', this.onMd.bind(this));
        this.canvas.addEvent('mouseup', this.onMu.bind(this));
        this.canvas.addEvent('mousewheel', this.onMw.bind(this));
        // 禁用浏览器右键菜单，防止遮挡 RDP 画面
        this.canvas.addEvent('contextmenu', function(e) {e.stop();});
        // 触摸设备支持
        if (this.uT) {
            this.canvas.addEvent('touchstart', this.onTs.bind(this));
            this.canvas.addEvent('touchend', this.onTe.bind(this));
            this.canvas.addEvent('touchmove', this.onTm.bind(this));
        }
        if (!this.cssC) {
            // 光标图片元素上也绑定相同事件，防止光标遮挡 Canvas 导致事件丢失
            this.cI.addEvent('mousemove', this.onMm.bind(this));
            this.cI.addEvent('mousedown', this.onMd.bind(this));
            this.cI.addEvent('mouseup', this.onMu.bind(this));
            this.cI.addEvent('mousewheel', this.onMw.bind(this));
            this.cI.addEvent('contextmenu', function(e) {e.stop();});
            if (this.uT) {
                this.cI.addEvent('touchstart', this.onTs.bind(this));
                this.cI.addEvent('touchend', this.onTe.bind(this));
                this.cI.addEvent('touchmove', this.onTm.bind(this));
            }
        }
        // 键盘事件绑定到 document，避免焦点丢失导致键盘输入丢失
        document.addEvent('keydown', this.onKd.bind(this));
        document.addEvent('keyup', this.onKu.bind(this));
        document.addEvent('keypress', this.onKp.bind(this));
        this.fireEvent('connected');
    },
    /**
     * WebSocket 连接关闭事件处理器（onclose）。
     * Chrome 特殊处理：该浏览器不触发 onerror，通过 wasClean 标志判断是否为连接失败。
     */
    onWSclose: function(evt) {
        if (Browser.name == 'chrome') {
            // Chrome 存在 bug：连接失败时不触发 WebSocket error 事件，
            // 改用 close 事件的 wasClean 标志判断
            if ((!evt.wasClean) && (!this.open)) {
                this.fireEvent('alert', 'Could not connect to WebSockets gateway');
            }
        }
        this.open = false;
        this._reset();
        this.fireEvent('disconnected');
    },
    /**
     * WebSocket 错误事件处理器（onerror）。
     * 连接阶段（CONNECTING 状态）发生错误时触发连接失败告警。
     */
    onWSerr: function (evt) {
        this.open = false;
        switch (this.sock.readyState) {
            case this.sock.CONNECTING:
                this.fireEvent('alert', 'Could not connect to WebSockets gateway');
                break;
        }
        this._reset();
    },
    /**
     * 将 Uint8Array 中的 RGBA 颜色值转换为 Canvas 可用的 CSS rgba() 字符串。
     * Alpha 通道从 0~255 归一化为 0~1。
     * @param {Uint8Array} c 含 [R, G, B, A] 四字节的数组
     * @returns {string} 如 'rgba(255,0,0,1)'
     */
    _c2s: function(c) {
        return 'rgba' + '(' + c[0] + ',' + c[1] + ',' + c[2] + ',' + ((0.0 + c[3]) / 255) + ')';
    },
    /**
     * 保存 Canvas 绘图状态并递增计数器，与 _ctxR 配对使用。
     */
    _ctxS: function() {
        this.cctx.save();
        this.ccnt += 1;
    },
    /**
     * 恢复上一次保存的 Canvas 绘图状态并递减计数器，与 _ctxS 配对使用。
     */
    _ctxR: function() {
        this.cctx.restore();
        this.ccnt -= 1;
    },
    /**
     * 将鼠标事件的按键信息转换为 RDP 鼠标标志位。
     * 若已设置人工鼠标标志（aMF），直接返回该值（触摸屏模拟鼠标按键时使用）。
     * @returns {number} 0x1000=左键, 0x2000=右键, 0x4000=中键
     */
    _mB: function(evt) {
        if (this.aMF) {
            return this.aMF; // 人工鼠标模式，使用预设按键标志
        }
        var bidx;
        if ('event' in evt && 'button' in evt.event) {
            bidx = evt.event.button; // W3C: 0=左键, 1=中键, 2=右键
        } else {
            bidx = evt.rightClick ? 2 : 0;
        }
        switch (bidx) {
            case 0:
                return 0x1000; // 左键
            case 1:
                return 0x4000; // 中键
            case 2:
                return 0x2000; // 右键
        }
        return 0x1000; // 默认左键
    },
    /**
     * 设置人工鼠标标志（用于触摸屏模拟鼠标按键）。
     * @param {Object|null} mf 含 r（右键）/m（中键）字段的对象；null 则清除
     */
    SetArtificialMouseFlags: function(mf) {
        if (null == mf) {
            this.aMF = 0; // 清除人工标志，恢复正常鼠标模式
            return;
        }
        this.aMF = 0x1000; // 默认左键
        if (mf.r) {
            this.aMF = 0x2000; // 右键
        }
        if (mf.m) {
            this.aMF = 0x4000; // 中键
        }
    },
    /**
     * 向服务端发送窗口大小调整请求（WSOP_CS_RESIZE）。
     * 服务端收到后重新协商 RDP 桌面分辨率。
     * @param {number} width  新宽度（像素）
     * @param {number} height 新高度（像素）
     */
    Resize: function(width, height) {
        if (this.sock && this.sock.readyState == this.sock.OPEN) {
            var buf = new ArrayBuffer(12);
            var a = new Uint32Array(buf);
            a[0] = 3; // WSOP_CS_RESIZE
            a[1] = width;
            a[2] = height;
            this.sock.send(buf);
        }
    }
});

/**
 * 将 RGBA 像素（4字节）从源数组复制到目标数组。
 * 优先使用 TypedArray.subarray/set 批量复制，不支持时逐字节复制。
 * @param {Uint8Array} inA  源数组
 * @param {number}     inI  源起始索引
 * @param {Uint8Array} outA 目标数组
 * @param {number}     outI 目标起始索引
 */
wsgate.copyRGBA = function(inA, inI, outA, outI) {
    if ('subarray' in inA) {
        outA.set(inA.subarray(inI, inI + 4), outI);
    } else {
        outA[outI++] = inA[inI++];
        outA[outI++] = inA[inI++];
        outA[outI++] = inA[inI++];
        outA[outI] = inA[inI];
    }
}
/**
 * 将源 RGBA 像素与一个 RGB565 像素值（16位）异或后写入目标数组。
 * 用于 RLE 压缩解码中的 FG/BG 混合：以上一行像素 XOR 前景色得到当前像素。
 * @param {Uint8Array} inA  上一行 RGBA 数组
 * @param {number}     inI  上一行对应像素起始索引
 * @param {Uint8Array} outA 当前行输出 RGBA 数组
 * @param {number}     outI 当前行写入索引
 * @param {number}     pel  前景色（16位 RGB565）
 */
wsgate.xorbufRGBAPel16 = function(inA, inI, outA, outI, pel) {
    var pelR = (pel & 0xF800) >> 11; // 提取 5 位红色分量
    var pelG = (pel & 0x7E0) >> 5;   // 提取 6 位绿色分量
    var pelB = pel & 0x1F;           // 提取 5 位蓝色分量
    // RGB565(5-6-5) -> RGB888(8-8-8) 位扩展
    pelR = (pelR << 3 & ~0x7) | (pelR >> 2);
    pelG = (pelG << 2 & ~0x3) | (pelG >> 4);
    pelB = (pelB << 3 & ~0x7) | (pelB >> 2);

    outA[outI++] = inA[inI++] ^ pelR;
    outA[outI++] = inA[inI++] ^ pelG;
    outA[outI++] = inA[inI] ^ pelB;
    outA[outI] = 255;                                 // Alpha 固定为不透明
}

/**
 * 将缓冲区中的 RGB565 小端字节对解码为 RGBA 写入目标数组。
 * @param {Uint8Array} inA  输入字节数组（每 2 字节为一个 RGB565 像素）
 * @param {number}     inI  输入起始索引
 * @param {Uint8Array} outA 输出 RGBA 数组
 * @param {number}     outI 输出起始索引
 */
wsgate.buf2RGBA = function(inA, inI, outA, outI) {
    // 小端字节序读取 16 位像素值
    var pel = inA[inI] | (inA[inI + 1] << 8);
    var pelR = (pel & 0xF800) >> 11;
    var pelG = (pel & 0x7E0) >> 5;
    var pelB = pel & 0x1F;
    // RGB565 -> RGB888
    pelR = (pelR << 3 & ~0x7) | (pelR >> 2);
    pelG = (pelG << 2 & ~0x3) | (pelG >> 4);
    pelB = (pelB << 3 & ~0x7) | (pelB >> 2);

    outA[outI++] = pelR;
    outA[outI++] = pelG;
    outA[outI++] = pelB;
    outA[outI] = 255;                    // Alpha 固定为不透明
}

/**
 * 将 16位 RGB565 像素值直接转换为 RGBA 写入目标数组。
 * @param {number}     pel  16位 RGB565 像素值
 * @param {Uint8Array} outA 输出 RGBA 数组
 * @param {number}     outI 输出起始索引（写入 4 字节）
 */
wsgate.pel2RGBA = function (pel, outA, outI) {
    var pelR = (pel & 0xF800) >> 11;
    var pelG = (pel & 0x7E0) >> 5;
    var pelB = pel & 0x1F;
    // RGB565 -> RGB888
    pelR = (pelR << 3 & ~0x7) | (pelR >> 2);
    pelG = (pelG << 2 & ~0x3) | (pelG >> 4);
    pelB = (pelB << 3 & ~0x7) | (pelB >> 2);

    outA[outI++] = pelR;
    outA[outI++] = pelG;
    outA[outI++] = pelB;
    outA[outI] = 255;                    // Alpha 固定为不透明
}

/**
 * 对 RGBA 像素数组进行垂直翻转（上下颠倒）。
 * RDP 位图以底部行优先（bottom-up）存储，需翻转后才能正确渲染到 Canvas。
 * @param {Uint8Array} inA    RGBA 像素数组（原地修改）
 * @param {number}     width  图像宽度（像素）
 * @param {number}     height 图像高度（像素）
 */
wsgate.flipV = function(inA, width, height) {
    var sll = width * 4;           // 每行字节数（4 字节/像素 × 宽度）
    var half = height / 2;         // 只需交换一半的行
    var lbot = sll * (height - 1); // 底部行起始偏移
    var ltop = 0;                  // 顶部行起始偏移
    var tmp = new Uint8Array(sll); // 临时行缓冲
    var i, j;
    if ('subarray' in inA) {
        // 使用 TypedArray 批量行交换（性能更好）
        for (i = 0; i < half ; ++i) {
            tmp.set(inA.subarray(ltop, ltop + sll));
            inA.set(inA.subarray(lbot, lbot + sll), ltop);
            inA.set(tmp, lbot);
            ltop += sll;
            lbot -= sll;
        }
    } else {
        // 回退到逐字节交换
        for (i = 0; i < half ; ++i) {
            for (j = 0; j < sll; ++j) {
                tmp[j] = inA[ltop + j];
                inA[ltop + j] = inA[lbot + j];
                inA[lbot + j] = tmp[j];
            }
            ltop += sll;
            lbot -= sll;
        }
    }
}

/**
 * 将无压缩的 RGB565 数据解码为 RGBA 像素数组。
 * @param {Uint8Array} inA      输入字节数组（每 2 字节为一个 RGB565 像素）
 * @param {number}     inLength 输入字节数
 * @param {Uint8Array} outA     输出 RGBA 数组（每 4 字节为一个像素）
 */
wsgate.dRGB162RGBA = function(inA, inLength, outA) {
    var inI = 0;
    var outI = 0;
    while (inI < inLength) {
        wsgate.buf2RGBA(inA, inI, outA, outI);
        inI += 2;  // 每个输入像素 2 字节（RGB565）
        outI += 4; // 每个输出像素 4 字节（RGBA）
    }
}

/**
 * 从 RLE 压缩流的订单头字节中提取操作码（Code ID）。
 * RDP NSCodec/RLE 压缩使用多种操作码，决定后续数据的解码方式。
 * @param {number} bOrderHdr 订单头字节
 * @returns {number} 操作码
 */
wsgate.ExtractCodeId = function(bOrderHdr) {
    var code;
    // 0xF0~0xFE 范围：操作码直接等于完整字节值
    switch (bOrderHdr) {
        case 0xF0:
        case 0xF1:
        case 0xF6:
        case 0xF8:
        case 0xF3:
        case 0xF2:
        case 0xF7:
        case 0xF4:
        case 0xF9:
        case 0xFA:
        case 0xFD:
        case 0xFE:
            return bOrderHdr;
    }
    code = bOrderHdr >> 5; // 高 3 位决定操作码（短格式）
    switch (code) {
        case 0x00:
        case 0x01:
        case 0x03:
        case 0x02:
        case 0x04:
            return code;
    }
    return bOrderHdr >> 4; // 高 4 位决定操作码（中等长度格式）
}

/**
 * 从 RLE 压缩流中提取当前操作码对应的运行长度（run length）。
 * 不同操作码有不同的长度编码规则（内嵌在头字节低位或跟随额外字节）。
 * @param {number}     code    操作码
 * @param {Uint8Array} inA     输入字节数组
 * @param {number}     inI     当前读取位置（指向头字节）
 * @param {Object}     advance 输出参数：advance.val 设为本次消耗的字节数
 * @returns {number} 解码出的运行长度（像素数）
 */
wsgate.ExtractRunLength = function(code, inA, inI, advance) {
    var runLength = 0;
    var ladvance = 1; // 默认消耗 1 字节（头字节本身）
    switch (code) {
        case 0x02:
            // 低 5 位为长度；为 0 则取下一字节 +1（扩展长度编码）
            runLength = inA[inI] & 0x1F;
            if (0 == runLength) {
                runLength = inA[inI + 1] + 1;
                ladvance += 1;
            } else {
                runLength *= 8;
            }
            break;
        case 0x0D:
            // 低 4 位为长度；为 0 则取下一字节 +1（扩展长度编码）
            runLength = inA[inI] & 0x0F;
            if (0 == runLength) {
                runLength = inA[inI + 1] + 1;
                ladvance += 1;
            } else {
                runLength *= 8;
            }
            break;
        case 0x00:
        case 0x01:
        case 0x03:
        case 0x04:
            // 低 5 位为长度；为 0 则取下一字节 +32（偏移扩展编码）
            runLength = inA[inI] & 0x1F;
            if (0 == runLength) {
                runLength = inA[inI + 1] + 32;
                ladvance += 1;
            }
            break;
        case 0x0C:
        case 0x0E:
            // 低 4 位为长度；为 0 则取下一字节 +16（偏移扩展编码）
            runLength = inA[inI] & 0x0F;
            if (0 == runLength) {
                runLength = inA[inI + 1] + 16;
                ladvance += 1;
            }
            break;
        case 0xF0:
        case 0xF1:
        case 0xF6:
        case 0xF8:
        case 0xF3:
        case 0xF2:
        case 0xF7:
        case 0xF4:
            // 长格式：后 2 字节（小端）为运行长度
            runLength = inA[inI + 1] | (inA[inI + 2] << 8);
            ladvance += 2;
            break;
    }
    advance.val = ladvance;
    return runLength;
}

/**
 * 将前景/背景位掩码图像（非首行）写入 RGBA 输出数组。
 * 掩码每个 bit 决定对应像素：1=前景色（XOR 上一行），0=背景色（复制上一行）。
 * @param {Uint8Array} outA     输出 RGBA 数组
 * @param {number}     outI     当前写入位置
 * @param {number}     rowDelta 上一行数据的字节偏移（= width * 4）
 * @param {number}     bitmask  8 位掩码
 * @param {number}     fgPel   前景色（16位 RGB565）
 * @param {number}     cBits   本次处理的像素数（通常为 8）
 * @returns {number} 更新后的写入位置
 */
wsgate.WriteFgBgImage16toRGBA = function(outA, outI, rowDelta, bitmask, fgPel, cBits) {
    var cmpMask = 0x01; // 从最低位开始逐位检查掩码

    while (cBits-- > 0) {
        if (bitmask & cmpMask) {
            // 该位为 1：前景色（XOR 上一行对应像素）
            wsgate.xorbufRGBAPel16(outA, outI - rowDelta, outA, outI, fgPel);
        } else {
            // 该位为 0：背景色（直接复制上一行对应像素）
            wsgate.copyRGBA(outA, outI - rowDelta, outA, outI);
        }
        outI += 4;
        cmpMask <<= 1; // 左移，检查下一位
    }
    return outI;
}

/**
 * 将前景/背景位掩码图像（首行）写入 RGBA 输出数组。
 * 首行无"上一行"可参考：掩码位 1=写前景色，0=写黑色（像素值 0）。
 * @param {Uint8Array} outA    输出 RGBA 数组
 * @param {number}     outI   当前写入位置
 * @param {number}     bitmask 8 位掩码
 * @param {number}     fgPel  前景色（16位 RGB565）
 * @param {number}     cBits  本次处理的像素数
 * @returns {number} 更新后的写入位置
 */
wsgate.WriteFirstLineFgBgImage16toRGBA = function(outA, outI, bitmask, fgPel, cBits) {
    var cmpMask = 0x01;

    while (cBits-- > 0) {
        if (bitmask & cmpMask) {
            // 该位为 1：写前景色
            wsgate.pel2RGBA(fgPel, outA, outI);
        } else {
            // 该位为 0：写黑色（0）
            wsgate.pel2RGBA(0, outA, outI);
        }
        outI += 4;
        cmpMask <<= 1;
    }
    return outI;
}

/**
 * 解码 RDP RLE16 压缩位图数据，输出为 RGBA 像素数组。
 *
 * RDP 使用 NSCodec RLE 对 16位 RGB565 位图进行压缩，支持多种操作码：
 *   0x00/0xF0 背景色游程：首行填黑，非首行复制上一行
 *   0x01/0xF1/0x0C/0xF6 前景色游程：XOR 前景色（0x0C/0xF6 携带新前景色）
 *   0x0E/0xF8 双色交替游程
 *   0x03/0xF3 颜色游程：重复单一颜色
 *   0x02/0xF2/0x0D/0xF7 FG/BG 掩码游程（0x0D/0xF7 携带新前景色）
 *   0x04/0xF4 原始像素游程：逐像素直接解码
 *   0xF9/0xFA 固定掩码特殊码；0xFD=白色像素；0xFE=黑色像素
 *
 * @param {Uint8Array} inA      压缩数据输入数组
 * @param {number}     inLength 压缩数据字节数
 * @param {number}     width    图像宽度（像素），用于计算行偏移和检测首行结束
 * @param {Uint8Array} outA     输出 RGBA 数组（调用方预分配 width*height*4 字节）
 */
wsgate.dRLE16_RGBA = function(inA, inLength, width, outA) {
    var runLength;
    var code, pixelA, pixelB, bitmask;
    var inI = 0;
    var outI = 0;
    var fInsertFgPel = false; // 是否在背景游程前插入一个前景像素
    var fFirstLine = true;    // 是否处于首行（首行无上一行可参考）
    var fgPel = 0xFFFFFF;     // 当前前景色（16位 RGB565，初始白色）
    var rowDelta = width * 4; // 上一行与当前行的字节偏移量
    var advance = {val: 0};   // 传出参数：ExtractRunLength 设置消耗的字节数

    while (inI < inLength) {
        // 检测是否已输出超过一行，离开首行范围
        if (fFirstLine) {
            if (outI >= rowDelta) {
                fFirstLine = false;
                fInsertFgPel = false;
            }
        }
        code = wsgate.ExtractCodeId(inA[inI]);
        if (code == 0x00 || code == 0xF0) {
            // 背景色游程：首行填黑（0），非首行复制上一行
            runLength = wsgate.ExtractRunLength(code, inA, inI, advance);
            inI += advance.val;
            if (fFirstLine) {
                if (fInsertFgPel) {
                    // 游程前先插入一个前景像素（RDP 协议规定）
                    wsgate.pel2RGBA(fgPel, outA, outI);
                    outI += 4;
                    runLength -= 1;
                }
                while (runLength > 0) {
                    wsgate.pel2RGBA(0, outA, outI); // 首行：填黑
                    runLength -= 1;
                    outI += 4;
                }
            } else {
                if (fInsertFgPel) {
                    wsgate.xorbufRGBAPel16(outA, outI - rowDelta, outA, outI, fgPel);
                    outI += 4;
                    runLength -= 1;
                }
                while (runLength > 0) {
                    wsgate.copyRGBA(outA, outI - rowDelta, outA, outI); // 非首行：复制上一行
                    runLength -= 1;
                    outI += 4;
                }
            }
            fInsertFgPel = true; // 下一个背景游程前需插入前景像素
            continue;
        }
        fInsertFgPel = false;
        switch (code) {
            case 0x01:
            case 0xF1:
            case 0x0C:
            case 0xF6:
                // 前景色游程：首行写前景色，非首行 XOR 上一行
                // 0x0C/0xF6 额外携带新的前景色（读 2 字节 RGB565）
                runLength = wsgate.ExtractRunLength(code, inA, inI, advance);
                inI += advance.val;
                if (code == 0x0C || code == 0xF6) {
                    fgPel = inA[inI] | (inA[inI + 1] << 8); // 更新前景色
                    inI += 2;
                }
                if (fFirstLine) {
                    while (runLength > 0) {
                        wsgate.pel2RGBA(fgPel, outA, outI);
                        runLength -= 1;
                        outI += 4;
                    }
                } else {
                    while (runLength > 0) {
                        wsgate.xorbufRGBAPel16(outA, outI - rowDelta, outA, outI, fgPel);
                        runLength -= 1;
                        outI += 4;
                    }
                }
                break;
            case 0x0E:
            case 0xF8:
                // 双色交替游程：每次写两个不同颜色像素交替输出
                runLength = wsgate.ExtractRunLength(code, inA, inI, advance);
                inI += advance.val;
                pixelA = inA[inI] | (inA[inI + 1] << 8);
                inI += 2;
                pixelB = inA[inI] | (inA[inI + 1] << 8);
                inI += 2;
                while (runLength > 0) {
                    wsgate.pel2RGBA(pixelA, outA, outI);
                    outI += 4;
                    wsgate.pel2RGBA(pixelB, outA, outI);
                    outI += 4;
                    runLength -= 1;
                }
                break;
            case 0x03:
            case 0xF3:
                // 颜色游程：重复写单一颜色（读 2 字节获取颜色）
                runLength = wsgate.ExtractRunLength(code, inA, inI, advance);
                inI += advance.val;
                pixelA = inA[inI] | (inA[inI + 1] << 8);
                inI += 2;
                while (runLength > 0) {
                    wsgate.pel2RGBA(pixelA, outA, outI);
                    outI += 4;
                    runLength -= 1;
                }
                break;
            case 0x02:
            case 0xF2:
            case 0x0D:
            case 0xF7:
                // FG/BG 掩码游程：每 8 个像素读一个掩码字节决定颜色
                // 0x0D/0xF7 额外携带新前景色
                runLength = wsgate.ExtractRunLength(code, inA, inI, advance);
                inI += advance.val;
                if (code == 0x0D || code == 0xF7) {
                    fgPel = inA[inI] | (inA[inI + 1] << 8);
                    inI += 2;
                }
                if (fFirstLine) {
                    while (runLength >= 8) {
                        bitmask = inA[inI++];
                        outI = wsgate.WriteFirstLineFgBgImage16toRGBA(outA, outI, bitmask, fgPel, 8);
                        runLength -= 8;
                    }
                } else {
                    while (runLength >= 8) {
                        bitmask = inA[inI++];
                        outI = wsgate.WriteFgBgImage16toRGBA(outA, outI, rowDelta, bitmask, fgPel, 8);
                        runLength -= 8;
                    }
                }
                if (runLength > 0) {
                    bitmask = inA[inI++];
                    if (fFirstLine) {
                        outI = wsgate.WriteFirstLineFgBgImage16toRGBA(outA, outI, bitmask, fgPel, runLength);
                    } else {
                        outI = wsgate.WriteFgBgImage16toRGBA(outA, outI, rowDelta, bitmask, fgPel, runLength);
                    }
                }
                break;
            case 0x04:
            case 0xF4:
                // 原始像素游程：逐像素从压缩流直接读取 RGB565 并解码
                runLength = wsgate.ExtractRunLength(code, inA, inI, advance);
                inI += advance.val;
                while (runLength > 0) {
                    wsgate.pel2RGBA(inA[inI] | (inA[inI + 1] << 8), outA, outI);
                    inI += 2;
                    outI += 4;
                    runLength -= 1;
                }
                break;
            case 0xF9:
                // 特殊码：写 8 像素，掩码固定为 0x03（bit0/bit1 为前景，其余为背景）
                inI += 1;
                if (fFirstLine) {
                    outI = wsgate.WriteFirstLineFgBgImage16toRGBA(outA, outI, 0x03, fgPel, 8);
                } else {
                    outI = wsgate.WriteFgBgImage16toRGBA(outA, outI, rowDelta, 0x03, fgPel, 8);
                }
                break;
            case 0xFA:
                // 特殊码：写 8 像素，掩码固定为 0x05（bit0/bit2 为前景）
                inI += 1;
                if (fFirstLine) {
                    outI = wsgate.WriteFirstLineFgBgImage16toRGBA(outA, outI, 0x05, fgPel, 8);
                } else {
                    outI = wsgate.WriteFgBgImage16toRGBA(outA, outI, rowDelta, 0x05, fgPel, 8);
                }
                break;
            case 0xFD:
                // 特殊码：写 1 个白色像素（0xFFFF = RGB565 全白）
                inI += 1;
                wsgate.pel2RGBA(0xFFFF, outA, outI);
                outI += 4;
                break;
            case 0xFE:
                // 特殊码：写 1 个黑色像素（0 = RGB565 全黑）
                inI += 1;
                wsgate.pel2RGBA(0, outA, outI);
                outI += 4;
                break;
        }
    }
}

