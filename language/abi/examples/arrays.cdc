fun adder(a: [Int]): Int? {

   if a.length == 0 {
    return nil
   }

   var i = 0
   var sum: Int = 0

   while i < a.length {
    sum = sum + a[i];
    i = i +1
   }
   return sum
}


fun zipOf3(a: [Any;3], b:[Int;3]): [[Any;2];3] {

    let c1: [Any;2] = [a[0], b[0]]
    let c2: [Any;2] = [a[1], b[1]]
    let c3: [Any;2] = [a[2], b[2]]

    let c: [[Any;2];3] = [c1, c2, c3]

    return c
}

fun callAll(functions fs: [(():Any?)]): [Any?] {
    var i = 0
    let ret: [Any?] = []

    while (i < fs.length) {
        ret.append(fs[i]())
        i=i+1
    }

    return ret
}

fun forEach(array: [Any?], f: ((Any?):Any?):[Any?] {
    var i = 0

    let ret: [Any?] = []

    while (i < array.length) {
        ret.append(f(array[i])
        i=i+1
    }

    return ret
}
