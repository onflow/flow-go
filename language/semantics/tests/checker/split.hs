testCases [] = []
testCases (name:lines) =
  let (body,rest) = span ((=="\t") . take 1) lines 
  in (name,unlines (map (drop 1) body)):testCases rest

main = do
  input <- getContents
  let tests = testCases (lines input)
  mapM_ print tests
