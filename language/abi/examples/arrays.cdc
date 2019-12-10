pub fun adder(a: [Int]): Int? {

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


pub fun zipOf3(a: [AnyStruct;3], b:[Int;3]): [[AnyStruct;2];3] {

    let c1: [AnyStruct;2] = [a[0], b[0]]
    let c2: [AnyStruct;2] = [a[1], b[1]]
    let c3: [AnyStruct;2] = [a[2], b[2]]

    let c: [[AnyStruct;2];3] = [c1, c2, c3]

    return c
}

pub fun callAll(functions fs: [(():AnyStruct?)]): [AnyStruct?] {
    var i = 0
    let ret: [AnyStruct?] = []

    while (i < fs.length) {
        ret.append(fs[i]())
        i=i+1
    }

    return ret
}

pub fun forEach(array: [AnyStruct?], f: ((AnyStruct?):AnyStruct?)):[AnyStruct?] {
    var i = 0

    let ret: [AnyStruct?] = []

    while (i < array.length) {
        ret.append(f(array[i]))
        i=i+1
    }

    return ret
}
