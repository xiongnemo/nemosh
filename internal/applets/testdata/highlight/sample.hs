{-# LANGUAGE OverloadedStrings #-}
module Main (main) where

import Data.List (sortBy)

{- outer {- inner -} still outer -}
data Tree a = Leaf | Node (Tree a) a (Tree a)
  deriving (Show, Eq)

insert :: Ord a => a -> Tree a -> Tree a
insert x Leaf = Node Leaf x Leaf
insert x t@(Node l v r)
  | x < v     = Node (insert x l) v r
  | otherwise = Node l v (insert x r)   -- a line comment

main :: IO ()
main = putStrLn "hello \"world\"" >> print (0x1F, 1.5e3, 'c')
