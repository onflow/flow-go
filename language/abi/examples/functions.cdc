pub fun sqrt(x x: Int): Int{
    return x*x
}

pub fun dividerProducer(divisor d: Int): ((Int):Int) {
    return fun(argument x: Int):Int {
        return x/d;
    }
}

pub fun decorator(function f: (():Void), before: (():Void)?, after: (():Void)?): (():Void) {
    return fun () {
        if let b = before {
            b()
        }
        f()
        if let a = after {
            a()
        }
    }
}
